package igngo

// Import this file's dependencies
import (
  "errors"
  "flag"
  "fmt"
  "io/ioutil"
  "log"
  "net/http"
  "time"
  "github.com/jinzhu/gorm"
  "github.com/auth0/go-jwt-middleware"
  "github.com/dgrijalva/jwt-go"
  _ "github.com/go-sql-driver/mysql"
)

type Server struct {
  /// Global database interface
  Db *gorm.DB

  // Port used for non-secure requests
  HttpPort string

  /// Auth0 public key used for token validation
  Auth0RsaPublickey string
}

/////////////////////////////////////////////////
/// Initialize this package
///
func Init(dbUserName, dbPassword, dbAddress, dbName string) (server *Server, err error) {

  var isGoTest bool

  server = &Server{
    HttpPort: ":8000",
  }

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
    log.Println("Running under go test")
    server.initTests()
  }
  return
}

// Run the router and server
func (s *Server) Run(routes Routes) {

  // JWT middlewares
  jwtOptionalMiddleware := jwtmiddleware.New(
    jwtmiddleware.Options{
      Debug:               false,

      // See https://github.com/auth0/go-jwt-middleware
      CredentialsOptional: true,

      SigningMethod:       jwt.SigningMethodRS256,

      ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
        // This method must return a public key or a secret, depending on the
        // chosen signing method
        pemKeyString := createPEMPublicKeyString(s.Auth0RsaPublickey)
        return jwt.ParseRSAPublicKeyFromPEM([]byte(pemKeyString))
      },
  })

  jwtRequiredMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
    Debug: false,
    SigningMethod: jwt.SigningMethodRS256,
    CredentialsOptional: false,
    ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
      // This method must return a public key or a secret, depending on the
      // chosen signing method
      pemKeyString := createPEMPublicKeyString(s.Auth0RsaPublickey)
      return jwt.ParseRSAPublicKeyFromPEM([]byte(pemKeyString))
    },
  })

  // Create the router
  router := NewRouter(routes, jwtOptionalMiddleware, jwtRequiredMiddleware)

  // Start the http webserver
  // Add some HTTP headers for handling preflight CORS requests.
  log.Fatal(http.ListenAndServe(s.HttpPort, router))
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
  url := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8&parseTime=True&loc=Local",
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
