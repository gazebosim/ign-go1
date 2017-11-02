package ign

// Import this file's dependencies
import (
  "errors"
  "flag"
  "fmt"
  "io/ioutil"
  "log"
  "net/http"
  "time"
  "github.com/gorilla/mux"
  "github.com/jinzhu/gorm"
  _ "github.com/go-sql-driver/mysql"
)

type Server struct {
  /// Global database interface
  Db *gorm.DB

  Router *mux.Router

  // Port used for non-secure requests
  HttpPort string

  // SSLport used for secure requests
  SSLport string

  // SSLCert is the path to the SSL certificate.
  SSLCert string

  // SSLKey is the path to the SSL private key.
  SSLKey string

  // DbConfig contains information about the database
  DbConfig DatabaseConfig

  // IsTest is true when tests are running.
  IsTest bool

  /// Auth0 public key used for token validation
  auth0RsaPublickey string
}

type DatabaseConfig struct {
  // Username to login to a database.
  UserName string
  // Password to login to a database.
  Password string
  // Address of the database.
  Address string
  // Name of the database.
  Name string
}

// gServer is an internal pointer to the Server.
var gServer *Server

/////////////////////////////////////////////////
/// Initialize this package
///
func Init(routes Routes, auth0RSAPublicKey string) (server *Server, err error) {

  server = &Server{
    HttpPort: ":8000",
    SSLport: ":4430",
  }
  server.readPropertiesFromEnvVars()
  gServer = server

  log.Printf("Parsed[%d]\n", flag.Parsed())

  /*if !flag.Parsed() {
    flag.Parse()
  }*/

  server.IsTest = flag.Lookup("test.v") != nil
  log.Printf("IsTest[%d]\n", server.IsTest)

  if server.IsTest {
    // Parse verbose setting, and adjust logging accordingly
    if !flag.Parsed() {
      flag.Parse()
    }

    v := flag.Lookup("test.v")
    isTestVerbose := v.Value.String() == "true"

    // Disable logging if needed
    if !isTestVerbose {
      log.SetFlags(0)
      log.SetOutput(ioutil.Discard)
    }
  }

  /*if !flag.Parsed() {
    flag.Parse()
  }

  v := flag.Lookup("test.v")
  server.IsTest =  v != nil && v.Value.String() == "true"
  log.Printf("IsTest[%d]\n", server.IsTest)

  // Parse verbose setting, and adjust logging accordingly
  if server.IsTest && v.Value.String() == "false" {
    log.SetFlags(0)
    log.SetOutput(ioutil.Discard)
  }*/

  // Initialize the database
  err = server.dbInit()

  if err != nil {
    log.Println(err)
  }

  if server.IsTest {
    server.initTests()
  } else {
    server.SetAuth0RsaPublicKey(auth0RSAPublicKey)
  }

  // Create the router
  server.Router = NewRouter(routes)

  return
}

// readPropertiesFromEnvVars configures the server based on env vars.
func (s *Server) readPropertiesFromEnvVars() error {
  var err error

  // Get the SSL certificate, if specified.
  if s.SSLCert, err = ReadEnvVar("IGN_SSL_CERT"); err != nil {
    log.Printf("Missing IGN_SSL_CER env variable. " +
               "Server will not be secure (no https).")
  }
  // Get the SSL private key, if specified.
  if s.SSLKey, err = ReadEnvVar("IGN_SSL_KEY"); err != nil {
    log.Printf("Missing IGN_SSL_KEY env variable. " +
               "Server will not be secure (no https).")
  }

  // Get the database username
  if s.DbConfig.UserName, err = ReadEnvVar("IGN_DB_USERNAME"); err != nil {
    log.Printf("Missing IGN_DB_USERNAME env variable. " +
      "Database connection will not work")
  }

  // Get the database password
  if s.DbConfig.Password, err = ReadEnvVar("IGN_DB_PASSWORD"); err != nil {
    log.Printf("Missing IGN_DB_PASSWORD env variable." +
               "Database connection will not work")
  }

  // Get the database address
  if s.DbConfig.Address, err = ReadEnvVar("IGN_DB_ADDRESS"); err != nil {
    log.Printf("Missing IGN_DB_ADDRESS env variable." +
               "Database connection will not work")
  }

  // Get the database name
  if s.DbConfig.Name, err = ReadEnvVar("IGN_DB_NAME"); err != nil {
    log.Printf("Missing IGN_DB_NAME env variable." +
               "Database connection will not work")
  }

  return nil
}

func (s *Server) Auth0RsaPublicKey() string {
  return s.auth0RsaPublickey
}

// Set the Server's Auth0 RSA public key
func (s *Server) SetAuth0RsaPublicKey(key string) {
  s.auth0RsaPublickey = key
  pemKeyString = "-----BEGIN CERTIFICATE-----\n" + s.auth0RsaPublickey +
         "\n-----END CERTIFICATE-----"
}

// Run the router and server
func (s *Server) Run() {

  if (s.SSLCert != "" && s.SSLKey != "") {
    // Start the webserver with TLS support.
    log.Fatal(http.ListenAndServeTLS(s.SSLport, s.SSLCert, s.SSLKey, s.Router))
  } else {
    // Start the http webserver
    log.Fatal(http.ListenAndServe(s.HttpPort, s.Router))
  }
}

/////////////////////////////////////////////////
// Private functions

// initTests is run as the last step of init() and only when `go test` was run.
func (s *Server) initTests() {
  // Override Auth0 public RSA key with test key, if present
  if testKey, err := ReadEnvVar("TEST_RSA256_PUBLIC_KEY"); err != nil {
    log.Printf("Missing TEST_RSA256_PUBLIC_KEY. Test with authentication may not work.")
  } else {
    s.SetAuth0RsaPublicKey(testKey)
  }
}

// DBInit Initialize the database connection
func (s *Server) dbInit() (error) {

  // Connect to the database
  url := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8&parseTime=True&loc=UTC",
    s.DbConfig.UserName, s.DbConfig.Password, s.DbConfig.Address,
    s.DbConfig.Name)

  var err error

  // Try to connect to the database. This is in for loop due to timing
  // issues. In particular, bitbucket pipelines uses a parallel database
  // container that may not be ready by the time this code executes.
  //
  // I have also seen this needed on amazon ec2 machines.
  for i := 0; i < 10; i++ {
    s.Db, err = gorm.Open("mysql", url)

    // Check for errors
    if err != nil {
      log.Printf("Attempt[%d] to connect to the database failed.\n", i)
      log.Println(url)
      log.Println(err)
      time.Sleep(5)
    } else {
      break
    }
  }

  if err != nil {
    return errors.New("Unable to connect to the database")
  }
  log.Printf("Connected to the database.\n")

  // Enable logging
  if flag.Lookup("test.v") != nil {
    s.Db.LogMode(flag.Lookup("test.v").Value.String() != "false")
  } else {
    s.Db.LogMode(true)
  }

  return nil
}
