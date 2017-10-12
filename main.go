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
  // "github.com/auth0/go-jwt-middleware"
  // "github.com/dgrijalva/jwt-go"
  _ "github.com/go-sql-driver/mysql"
)

type Server struct {
  /// Global database interface
  Db *gorm.DB

  Router *mux.Router

  // Port used for non-secure requests
  HttpPort string

  /// Auth0 public key used for token validation
  Auth0RsaPublickey string
}

// gServer is an internal pointer to the Server.
var gServer *Server

/////////////////////////////////////////////////
/// Initialize this package
///
func Init(dbUserName, dbPassword, dbAddress, dbName string, routes Routes, auth0RSAPublicKey string) (server *Server, err error) {

  var isGoTest bool

  server = &Server{
    HttpPort: ":8000",
  }
  gServer = server

  isGoTest = flag.Lookup("test.v") != nil

  // Parse verbose setting, and adjust logging accordingly
  if isGoTest {
    if !flag.Parsed() {
      flag.Parse()
    }

    if flag.Lookup("test.v").Value.String() == "false" {
      log.SetFlags(0)
      log.SetOutput(ioutil.Discard)
    }
  }

  // Initialize the database
  server.Db, err = dbInit(dbUserName, dbPassword, dbAddress, dbName)

  if err != nil {
    log.Println(err)
  }

  if isGoTest {
    server.initTests()
  } else {
    server.Auth0RsaPublickey = auth0RSAPublicKey
  }

  pemKeyString = "-----BEGIN CERTIFICATE-----\n" + server.Auth0RsaPublickey +
         "\n-----END CERTIFICATE-----"

  // Create the router
  server.Router = NewRouter(routes)

  return
}

// Run the router and server
func (s *Server) Run() {

  // Start the http webserver
  // Add some HTTP headers for handling preflight CORS requests.
  log.Fatal(http.ListenAndServe(s.HttpPort, s.Router))
}

/////////////////////////////////////////////////
// Private functions

// initTests is run as the last step of init() and only when `go test` was run.
func (s *Server) initTests() {
  // Override Auth0 public RSA key with test key, if present
  if testKey, err := ReadEnvVar("TEST_RSA256_PUBLIC_KEY"); err != nil {
    log.Printf("Missing TEST_RSA256_PUBLIC_KEY. Test with authentication may not work.")
  } else {
    s.Auth0RsaPublickey = testKey
  }
}

// DBInit Initialize the database connection
func dbInit(username, password, address, database string) (*gorm.DB, error) {

  // Connect to the database
  url := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8&parseTime=True&loc=UTC",
    username, password, address, database)

  var err error
  var db *gorm.DB

  // Try to connect to the database. This is in for loop due to timing
  // issues. In particular, bitbucket pipelines uses a parallel database
  // container that may not be ready by the time this code executes.
  //
  // I have also seen this needed on amazon ec2 machines.
  for i := 0; i < 10; i++ {
    db, err = gorm.Open("mysql", url)

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
    return nil, errors.New("Unable to connect to the database")

  log.Printf("Connected to the database.\n")
  }

  // Enable logging
  if flag.Lookup("test.v") != nil {
    db.LogMode(flag.Lookup("test.v").Value.String() != "false")
  } else {
    db.LogMode(true)
  }

  return db, nil
}
