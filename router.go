package ign

import (
  "encoding/json"
  "fmt"
  "log"
  "net/http"
  "reflect"
  "regexp"
  "sort"
  "strings"
  "time"
  "github.com/auth0/go-jwt-middleware"
  "github.com/codegangsta/negroni"
  "github.com/dgrijalva/jwt-go"
  "github.com/golang/protobuf/proto"
  "github.com/gorilla/mux"
  "github.com/jpillora/go-ogle-analytics"
)

// Detail stores information about a paramter.
type Detail struct {
  Type        string `json:"type"`
  Description string `json:"description"`
  Required    bool   `json:"required"`
}

// Header stores the information about headers included in a request.
type Header struct {
  Name          string `json:"name"`
  HeaderDetails Detail `json:"details"`
}

// Handler represents an HTTP Handler that can also return a ErrMsg
// See https://blog.golang.org/error-handling-and-go
type Handler func(http.ResponseWriter, *http.Request) *ErrMsg

// HandlerWithResult represents an HTTP Handler that that has a result
type HandlerWithResult func(w http.ResponseWriter, r *http.Request) (interface{}, *ErrMsg)

// FormatHandler represents a format type string, and handler function pair. Handlers are called in response to a route request.
type FormatHandler struct {
  // Format (eg: .json, .proto, .html)
  Extension string `json:"extension"`

  // Processor for the url pattern
  Handler http.Handler `json:"-"`
}

// TypeJSONResult represents a function result that can be exported to JSON
type TypeJSONResult struct {
  wrapperField string
  fn HandlerWithResult
}

// ProtoResult provides protobuf serialization for handler results
type ProtoResult HandlerWithResult

// FormatHandlers is a slice of FormatHandler values.
type FormatHandlers []FormatHandler

// Method associates an HTTP method (GET, POST, PUT, DELETE) with a list of
// handlers.
type Method struct {
  // GET, POST, PUT, DELETE
  // \todo: Make this an enum
  Type string `json:"type"`

  // Description of the method
  Description string `json:"description"`

  // A slice of hanlders used to process this method.
  Handlers FormatHandlers `json:"handler"`
}

// Methods is a slice of Method.
type Methods []Method

// SecureMethods is a slice of Method that require authentication.
type SecureMethods []Method

// Route is a definition of a route
type Route struct {

  // Name of the route
  Name string `json:"name"`

  // Description of the route
  Description string `json:"description"`

  // URI pattern
  URI string `json:"uri"`

  // Headers required by the route
  Headers []Header `json:"headers"`

  // HTTP methods supported by the route
  Methods Methods `json:"methods"`

  // Secure HTTP methods supported by the route
  SecureMethods SecureMethods `json:"secure_methods"`
}

// Routes is an array of Route
type Routes []Route


// AuthHeadersRequired is an array of Headers needed when authentication is
// required.
var AuthHeadersRequired = []Header {
  {
    Name: "authorization: Bearer <YOUR_JWT_TOKEN>",
    HeaderDetails: Detail {
      Required: true,
    },
  },
}

// AuthHeadersOptional is an array of Headers needed when authentication is
// optional.
var AuthHeadersOptional = []Header {
  {
    Name: "authorization: Bearer <YOUR_JWT_TOKEN>",
    HeaderDetails: Detail {
      Required: false,
    },
  },
}

// NewRouter creates a new Gorilla/mux router
func NewRouter(routes Routes) *mux.Router {

  // We need to set StrictSlash to "false" (default) to avoid getting
  // routes redirected automatically.
  router := mux.NewRouter().StrictSlash(false)

  // Process the routes defined in routes.go
  for routeIndex, route := range routes {

    var allowedOptions []string

    // Process unsecure routes
    for _, method := range route.Methods {
      for _, formatHandler := range method.Handlers {
        createRouteHelper(router, &routes, routeIndex, method.Type, false,
                          &allowedOptions, formatHandler)
      }
    }

    // Process secure routes
    for _, method := range route.SecureMethods {
      for _, formatHandler := range method.Handlers {
        createRouteHelper(router, &routes, routeIndex, method.Type, true,
                          &allowedOptions, formatHandler)
      }
    }
  }
  // NOTE: sortedREs and corsMap are private vars defined below
  // Sorting corsMap is needed to correctly resolve OPTION requests
  // that need to match a regex.
  sortedREs = getSortedREs(corsMap)

  return router
}

// JSONResult provides JSON serialization for handler results
func JSONResult(handler HandlerWithResult) TypeJSONResult {
  return TypeJSONResult{"", handler}
}

// JSONListResult provides JSON serialization for handler results that are
// slices of objects.
func JSONListResult(wrapper string, handler HandlerWithResult) TypeJSONResult {
  return TypeJSONResult{wrapper, handler}
}

/////////////////////////////////////////////////
func (fn Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  if err := fn(w, r); err != nil {
    reportJSONError(w, *err)
  }
}

/////////////////////////////////////////////////
func (t TypeJSONResult) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  result, err := t.fn(w, r)
  if err != nil {
    reportJSONError(w, *err)
    return
  }

  var data interface{}
  // Is there any wrapper field to cut off ?
  if t.wrapperField != "" {
    value := reflect.ValueOf(result)
    fieldValue := reflect.Indirect(value).FieldByName(t.wrapperField)
    data = fieldValue.Interface()
    // If the underlying data is an empty slice then force the creation of
    // an empty json `[]` as output
    if fieldValue.Kind() == reflect.Slice && fieldValue.Len() == 0 {
      data = make([]string, 0)
    }
  } else {
    data = result
  }
  w.Header().Set("Content-Type", "application/json")
  // Marshal the response into a JSON
  if err := json.NewEncoder(w).Encode(data); err != nil {
    em := NewErrorMessageWithBase(ErrorMarshalJSON, err)
    reportJSONError(w, *em)
    return
  }
}

/////////////////////////////////////////////////
func (fn ProtoResult) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  result, err := fn(w, r)
  if err != nil {
    reportJSONError(w, *err)
    return
  }

  // Marshal the protobuf data and write it out.
  var pm = result.(proto.Message)
  data, e := proto.Marshal(pm)
  if e != nil {
    em := NewErrorMessageWithBase(ErrorMarshalProto, e)
    reportJSONError(w, *em)
    return
  }
  w.Header().Set("Content-Type", "application/arraybuffer")
  w.Write(data)
}

/////////////////////////////////////////////////
// Private members
/////////////////////////////////////////////////

var corsMap = map[string]int{}
// sortedREs keeps a sorted list of registered routes in corsMap.
// It allows us to iterate the corsMap in 'order'.
var sortedREs []string

var pemKeyString string

// JWT middlewares
var jwtOptionalMiddleware = jwtmiddleware.New(
  jwtmiddleware.Options{
    Debug:               false,

    // See https://github.com/auth0/go-jwt-middleware
    CredentialsOptional: true,

    SigningMethod:       jwt.SigningMethodRS256,

    ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
      // This method must return a public key or a secret, depending on the
      // chosen signing method
      return jwt.ParseRSAPublicKeyFromPEM([]byte(pemKeyString))
    },
})

var jwtRequiredMiddleware = jwtmiddleware.New(jwtmiddleware.Options{
  Debug: false,
  SigningMethod: jwt.SigningMethodRS256,
  CredentialsOptional: false,
  ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
    // This method must return a public key or a secret, depending on the
    // chosen signing method
    return jwt.ParseRSAPublicKeyFromPEM([]byte(pemKeyString))
  },
})

/////////////////////////////////////////////////
// sortRE is an internal []string wrapper type used to sort by
// the number of "[^/]+" string occurrences found in a regex (ie. count).
// If the same count is found then the larger string will take precedence.
type sortRE []string
func (s sortRE) Len() int {
  return len(s)
}
func (s sortRE) Swap(i, j int) {
  s[i], s[j] = s[j], s[i]
}
func (s sortRE) Less(i, j int) bool {
  ci := strings.Count(s[i], "[^/]+")
  cj := strings.Count(s[j], "[^/]+")
  if ci == cj {
    return len(s[i]) > len(s[j])
  }
  return ci < cj
}

func getSortedREs(m map[string]int) []string {
  var keys []string
  for k := range m {
    keys = append(keys, k)
  }
  sort.Sort(sortRE(keys))
  return keys
}

/////////////////////////////////////////////////
// Helper function that creates a route
func createRouteHelper(router *mux.Router, routes *Routes,
                       routeIndex int, methodType string, secure bool,
                       allowedOptions *[]string, formatHandler FormatHandler) {

  *allowedOptions = append(*allowedOptions, methodType)
  handler := formatHandler.Handler

  // Configure auth middleware
  var authMiddleware negroni.HandlerFunc
  if !secure {
    authMiddleware = negroni.HandlerFunc(jwtOptionalMiddleware.HandlerWithNext)
  } else {
    authMiddleware = negroni.HandlerFunc(jwtRequiredMiddleware.HandlerWithNext)
  }

  routeName := (*routes)[routeIndex].Name

  // Configure middlewares chain
  handler = negroni.New(
    negroni.HandlerFunc(panicRecoveryMiddleware),
    negroni.HandlerFunc(requireDBMiddleware),
    negroni.HandlerFunc(addCORSheadersMiddleware),
    authMiddleware,
    negroni.HandlerFunc(newGaEventTracking(routeName)),
    negroni.Wrap(http.Handler(handler)),
  )

  // Last, wrap everything with a Logger middleware
  handler = logger(handler, routeName)

  uriPath := (*routes)[routeIndex].URI + formatHandler.Extension

  // Create the route handler.
  router.
  Methods(methodType).
  Path(uriPath).
  Name(routeName + formatHandler.Extension).
  Handler(handler)

  // Setup a regular expression for "{_text_}" URL parameters.
  re := regexp.MustCompile("{.+?}")

  // Store route information for options
  reString := re.ReplaceAllString(strings.Replace(uriPath, ".", "\\.", -1), "[^/]+")
  corsMap[reString] = routeIndex

  // Create the OPTIONS route handler.
  // Added the HTTP method "OPTIONS" to each route,
  // to handle CORS preflight requests.
  router.
  Methods("OPTIONS").
  Path(uriPath).
  Name(routeName + formatHandler.Extension).
  Handler(http.HandlerFunc(
    func(w http.ResponseWriter, r *http.Request) {
      index := 0
      ok := false
      // Find the matching URL
      for _, key := range sortedREs {
        // Make sure the regular expression matches the complete URL path
        if regexp.MustCompile(key).FindString(r.URL.Path) == r.URL.Path {
          ok = true
          index = corsMap[key]
          break
        }
      }

      if (ok) {
        if output, e := json.Marshal((*routes)[index]); e != nil {
          err := NewErrorMessageWithBase(ErrorMarshalJSON, e)
          reportJSONError(w, *err)
        } else {
          w.Header().Set("Allow", strings.Join((*allowedOptions)[:], ","))
          w.Header().Set("Content-Type", "application/json")
          addCORSheaders(w)
          fmt.Fprintln(w, string(output))
        }
        return
      }

      // Return error if a URL did not match
      err := ErrorMessage(ErrorNameNotFound)
      reportJSONError(w, err)
    }))
}


/////////////////////////////////////////////////
// Middleware to ensure the DB instance exists.
// By having this middleware, then any route handler can safely assume the DB
// is present.
func requireDBMiddleware(w http.ResponseWriter, r *http.Request,
                      next http.HandlerFunc) {
  if gServer.Db == nil {
    errMsg := ErrorMessage(ErrorNoDatabase)
    reportJSONError(w, errMsg)
  } else {
    next(w, r)
  }
}

/////////////////////////////////////////////////
// Panic-Recover middleware to avoid Crashing the server
// on unexpected panicking.
// See https://blog.golang.org/defer-panic-and-recover
func panicRecoveryMiddleware(w http.ResponseWriter, r *http.Request,
                          next http.HandlerFunc) {

  defer func() {
    if err := recover(); err != nil {
      log.Printf("Recovered from panic: %+v", err)
      http.Error(w, http.StatusText(500), 500)
    }
  }()

  next(w, r)
}

/////////////////////////////////////////////////
func addCORSheadersMiddleware(w http.ResponseWriter, r *http.Request,
                              next http.HandlerFunc) {
  addCORSheaders(w)
  next(w, r)
}

// addCORSheaders adds the required Access Control headers to the HTTP response
func addCORSheaders(w http.ResponseWriter) {
  w.Header().Set("Access-Control-Allow-Methods",
                 "GET, HEAD, POST, PUT, PATCH, DELETE")

  w.Header().Set("Access-Control-Allow-Credentials", "true")

  w.Header().Set("Access-Control-Allow-Headers",
                 `Accept, Accept-Language, Content-Language, Origin,
                  Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token,
                  Authorization`)
  w.Header().Set("Access-Control-Allow-Origin", "*")

  w.Header().Set("Access-Control-Expose-Headers","Link, X-Total-Count")
}

/////////////////////////////////////////////////
// ReportJSONError logs an error message and return an HTTP error including
// JSON payload
func reportJSONError(w http.ResponseWriter, errMsg ErrMsg) {
  log.Println("Error in [" + Trace() + "]\n\t" + errMsg.LogString())
  if errMsg.BaseError != nil {
    log.Printf("Base error: %v", errMsg.BaseError)
  }

  output, err := json.Marshal(errMsg);
  if err != nil {
    reportError(w, "Unable to marshal JSON", http.StatusServiceUnavailable)
    return
  }

  http.Error(w, string(output), errMsg.StatusCode)
}

/////////////////////////////////////////////////
// reportError logs an error message and return an HTTP error
func reportError(w http.ResponseWriter, msg string, errCode int) {
  log.Println("Error in [" + Trace() + "]\n\t" + msg)
  http.Error(w, msg, errCode)
}

/////////////////////////////////////////////////
// logger is a decorator used to output HTTP requests.
func logger(inner http.Handler, name string) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    start := time.Now()

    inner.ServeHTTP(w, r)

    log.Printf(
      "%s\t%s\t%s\t%s",
      r.Method,
      r.RequestURI,
      name,
      time.Since(start),
    )
  })
}

/////////////////////////////////////////////////
// gaEventTracking is a middleware to send events to Google Analytics.
// Events will be automatically created using route information.
// This middleware requires IGN_GA_TRACKING_ID and IGN_GA_APP_NAME
// env vars.
func newGaEventTracking(routeName string) negroni.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
    next(w, r)

    // Track event with GA, if enabled
    if gServer.GaAppName == "" || gServer.GaTrackingID == "" {
      return
    }
    c, err := ga.NewClient(gServer.GaTrackingID)
    if err != nil {
      fmt.Println("Error creating GA client", err)
      return
    }
    c.DataSource(gServer.GaAppName)
    c.ApplicationName(gServer.GaAppName)
    cat := gServer.GaCategoryPrefix + routeName
    action := r.Method
    e := ga.NewEvent(cat, action).Label(r.URL.String())
    if err := c.Send(e); err != nil {
      fmt.Println("Error while sending event to GA", err)
    }
  }
}
