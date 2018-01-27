package ign

// Import this file's dependencies
import (
  "errors"
  "flag"
  "fmt"
  "io/ioutil"
  "log"
  "net/http"
  "strconv"
  "time"
  "github.com/gorilla/mux"
  "github.com/jinzhu/gorm"
  // Needed by dbInit
  _ "github.com/go-sql-driver/mysql"
)

// Server encapsulates information needed by a downstream application
type Server struct {
  /// Global database interface
  Db *gorm.DB

  Router *mux.Router

  // Port used for non-secure requests
  HTTPPort string

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

// DatabaseConfig contains information about a database connection
type DatabaseConfig struct {
  // Username to login to a database.
  UserName string
  // Password to login to a database.
  Password string
  // Address of the database.
  Address string
  // Name of the database.
  Name string
  // Allowed Max Open Connections
  // See https://golang.org/src/database/sql/sql.go
  MaxOpenConns int
}

// gServer is an internal pointer to the Server.
var gServer *Server

// Init initialize this package
func Init(routes Routes, auth0RSAPublicKey string) (server *Server, err error) {

  server = &Server{
    HTTPPort: ":8000",
    SSLport: ":4430",
  }
  server.readPropertiesFromEnvVars()
  gServer = server

  server.IsTest = flag.Lookup("test.v") != nil

  if server.IsTest {
    // Let's use a separate DB name if under test mode.
    server.DbConfig.Name = server.DbConfig.Name + "_test"

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
    log.Printf("Missing IGN_SSL_CERT env variable. " +
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

  // Get the database max open conns
  var maxStr string
  if maxStr, err = ReadEnvVar("IGN_DB_MAX_OPEN_CONNS"); err != nil {
    log.Printf("Missing IGN_DB_MAX_OPEN_CONNS env variable." +
               "Database max open connections will not be set," +
              "with risk of getting 'too many connections' error.")
    s.DbConfig.MaxOpenConns = 0
  } else {
    var i int64
    i, err = strconv.ParseInt(maxStr, 10, 32)
    if err != nil || i == 0 {
      log.Printf("Error parsing IGN_DB_MAX_OPEN_CONNS env variable." +
          "Database max open connections will not be set," +
          "with risk of getting 'too many connections' error.")
        s.DbConfig.MaxOpenConns = 0
    } else {
      s.DbConfig.MaxOpenConns = int(i)
    }
  }

  return nil
}

// Auth0RsaPublicKey return the Auth0 public key
func (s *Server) Auth0RsaPublicKey() string {
  return s.auth0RsaPublickey
}

// SetAuth0RsaPublicKey sets the server's Auth0 RSA public key
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
    log.Fatal(http.ListenAndServe(s.HTTPPort, s.Router))
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

// dbInit Initialize the database connection
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
    s.Db = nil
    return errors.New("Unable to connect to the database")
  }
  log.Printf("Connected to the database.\n")

  // Enable logging
  if flag.Lookup("test.v") != nil {
    s.Db.LogMode(flag.Lookup("test.v").Value.String() != "false")
  } else {
    s.Db.LogMode(true)
  }

  // Set max open connections in pool. Other requests will be automatically queued
  // by go/sql. See https://golang.org/src/database/sql/sql.go
  if s.DbConfig.MaxOpenConns != 0 {
    log.Println("Setting DB Max Open Conns", s.DbConfig.MaxOpenConns)
    s.Db.DB().SetMaxOpenConns(s.DbConfig.MaxOpenConns)
  }

  return nil
}
