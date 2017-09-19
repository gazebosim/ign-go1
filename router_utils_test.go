package igngo

import (
  "bytes"
  "encoding/json"
  "fmt"
  "io/ioutil"
  "log"
  "net/http"
  "net/http/httptest"
  "os"
  "strings"
  "testing"
  "github.com/gorilla/mux"
)

var router *mux.Router

// setup helper function
func setup() {
  // Make sure we don't have data from other tests.
  // For this we drop db tables and recreate them.
  cleanDBTables()

  // Check for auth0 environment variables.
  if os.Getenv("IGN_FUEL_TEST_JWT") == "" {
    log.Printf("Missing IGN_FUEL_TEST_JWT env variable." +
               "Authentication will not work.")
  }

  // Create the router, and indicate that we are testing
  router = NewRouter(true)
}

// assertRoute is a helper function that checks for a valid route
// \param[in] method One of "GET", "PATCH", "PUT", "POST", "DELETE", "OPTIONS"
// \param[in] route The URL string
// \param[in] code The expected result HTTP code
// \param[in] t Testing pointer
// \return[out] *[]byte A pointer to a bytes slice containing the response body.
// \return[out] bool A flag indicating if the operation was ok.
func assertRoute(method string, route string, code int, t *testing.T) (*[]byte, bool) {
  return assertRouteWithBody(method, route, nil, code, t)
}

// \return[out] *[]byte A pointer to a bytes slice containing the response body.
// \return[out] bool A flag indicating if the operation was ok.
func assertRouteWithBody(method string, route string, body *bytes.Buffer, code int, t *testing.T) (*[]byte, bool) {
  jwt := os.Getenv("IGN_FUEL_TEST_JWT")
  return assertRouteMultipleArgs(method, route, body, code, &jwt, "application/json", t)
}

// Helper function that checks for a valid route.
// \param[in] method One of "GET", "PATCH", "PUT", "POST", "DELETE"
// \param[in] route The URL string
// \param[in] body The body to send in the request, or nil
// \param[in] code The expected response HTTP code
// \param[in] signedToken JWT token as base64 string, or nil.
// \param[in] contentType The expected response content type
// \param[in] t Test pointer
// \return[out] *[]byte A pointer to a bytes slice containing the response body.
// \return[out] bool A flag indicating if the operation was ok.
func assertRouteMultipleArgs(method string, route string, body *bytes.Buffer, code int, signedToken *string, contentType string, t *testing.T) (*[]byte, bool) {
  var ok bool
  var b []byte

  var buff bytes.Buffer
  if body != nil {
    buff = *body
  }
  // Create a new http request
  req, err := http.NewRequest(method, route, &buff)

  // Add the authorization token
  if signedToken != nil {
    req.Header.Set("Authorization", "Bearer " + *signedToken)
  }

  // Make sure the request was generated
  if err != nil {
    t.Fatal("Request failed!")
    return &b, ok
  }

  // Process the request
  respRec := httptest.NewRecorder()
  router.ServeHTTP(respRec, req)

  // Read the result
  var er error
  if b, er = ioutil.ReadAll(respRec.Body); er != nil {
    t.Fatal("Failed to read the server response")
    return &b, ok
  }

  // Make sure the error code is correct
  if respRec.Code != code {
    t.Fatalf("Server error: returned %d instead of %d. Route: %s", respRec.Code, code, route)
    return &b, ok
  }

  if strings.Compare(respRec.Header().Get("Content-Type"), contentType) != 0 {
    t.Fatal("Expected Content-Type[", contentType, "] != [",
            respRec.Header().Get("Content-Type"), "]")
    return &b, ok
  }
  ok = true
  return &b, ok
}

// This function tries to unmarshal a backend's ErrMsg and compares to given ErrCode
func assertBackendErrorCode(bslice *[]byte, errCode int, t *testing.T) {
  var errMsg ErrMsg
  if err := json.Unmarshal(*bslice, &errMsg); err != nil {
    t.Fatal("Unable to unmarshal bytes slice", err, string(*bslice))
  }
  if errMsg.ErrCode != errCode {
    t.Fatal("[ErrCode] is different than expected code", errMsg.ErrCode, errCode)
  }
}
