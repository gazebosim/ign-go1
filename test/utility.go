package igntest

import (
  "encoding/json"
  "net/http"
  "bytes"
  "io"
  "log"
  "mime/multipart"
  "net/http/httptest"
  "os"
  "path/filepath"
  "io/ioutil"
  "testing"
  "sort"
  "strings"
  "github.com/gorilla/mux"
  "bitbucket.org/ignitionrobotics/ign-go"
)

var router *mux.Router

// FileDesc describes a file to be created. It is used by
// func CreateTmpFolderWithContents and sendMultipartPOST.
// Fields:
// path: is the file path to be sent in the multipart form.
// contents: is the string contents to write in the file. Note: if contents
// value is ":dir" then a Directory will be created instead of a File. This is only
// valid when used with CreateTmpFolderWithContents func.
type FileDesc struct {
  Path string
  Contents string
}

// setup helper function
func Setup(_router *mux.Router) {
  router = _router
}

// SendMultipartPOST executes a multipart POST request with the given form
// fields and multipart files, and returns the received http status code,
// the response body, and a success flag.
func SendMultipartPOST(t *testing.T, uri string, jwt string,
  params map[string]string, files []FileDesc) (respCode int,
  bslice *[]byte, ok bool) {

  body := &bytes.Buffer{}
  writer := multipart.NewWriter(body)
  for _, fd := range files {
    // Remove base path
    part, err := writer.CreateFormFile("file", fd.Path)
    if err != nil {
      t.Fatal("Could not create FormFile", fd.Path, err)
      return
    }
    _, err = io.WriteString(part, fd.Contents)
  }

  for key, val := range params {
    _ = writer.WriteField(key, val)
  }
  if err := writer.Close(); err != nil {
    t.Fatal("Could not close multipart form writer", err)
    return
  }

  req, err := http.NewRequest("POST", uri, body)
  if err != nil {
    t.Fatal("Could not create POST request", err)
    return
  }
  // Adds the "Content-Type: multipart/form-data" header.
  req.Header.Add("Content-Type", writer.FormDataContentType())

  if jwt != "" {
    // Add the authorization token
    req.Header.Set("Authorization", "Bearer " + jwt)
  }

  // Process the request
  respRec := httptest.NewRecorder()
  router.ServeHTTP(respRec, req)

  // Process results
  respCode = respRec.Code

  var b []byte
  var er error
  b, er = ioutil.ReadAll(respRec.Body)
  if er != nil {
    t.Fatal("Failed to read the server response", er)
    return
  }

  bslice = &b
  ok = true
  return
}


// CreateTmpFolderWithContents creates a tmp folder with the given files and
// returns the path to the created folder. See type fileDesc above.
func CreateTmpFolderWithContents(folderName string, files []FileDesc) (string, error) {
  baseDir, err := ioutil.TempDir("", folderName)
  if err != nil {
    return "", err
  }

  for _, fd := range files {
    fullpath := filepath.Join(baseDir, fd.Path)
    dir := filepath.Dir(fullpath)
    if dir != baseDir {
      if err := os.MkdirAll(dir, os.ModePerm); err != nil {
        return "", err
      }
    }

    if (fd.Contents == ":dir") {
      // folder
      if err := os.MkdirAll(fullpath, os.ModePerm); err != nil {
        return "", err
      }
    } else {
      // normal file with given contents
      f, err := os.Create(fullpath)
      defer f.Close()
      if err != nil {
        log.Println("Unable to create [" + fullpath + "]")
        return "", err
      }
      if _, err := f.WriteString(fd.Contents); err != nil {
        log.Println("Unable to write contents to [" + fullpath + "]")
        return "", err
      }
      f.Sync()
    }
  }
  return baseDir, nil
}

// assertRoute is a helper function that checks for a valid route
// \param[in] method One of "GET", "PATCH", "PUT", "POST", "DELETE", "OPTIONS"
// \param[in] route The URL string
// \param[in] code The expected result HTTP code
// \param[in] t Testing pointer
// \return[out] *[]byte A pointer to a bytes slice containing the response body.
// \return[out] bool A flag indicating if the operation was ok.
func AssertRoute(method, route string, code int, t *testing.T) (*[]byte, bool) {
  return AssertRouteWithBody(method, route, nil, code, t)
}

// \return[out] *[]byte A pointer to a bytes slice containing the response body.
// \return[out] bool A flag indicating if the operation was ok.
func AssertRouteWithBody(method, route string, body *bytes.Buffer, code int, t *testing.T) (*[]byte, bool) {
  jwt := os.Getenv("IGN_TEST_JWT")
  return AssertRouteMultipleArgs(method, route, body, code, &jwt,
                                 "application/json", t)
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
func AssertRouteMultipleArgs(method string, route string, body *bytes.Buffer, code int, signedToken *string, contentType string, t *testing.T) (*[]byte, bool) {
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
func AssertBackendErrorCode(testName string, bslice *[]byte, errCode int, t *testing.T) {
  var errMsg ign.ErrMsg
  if err := json.Unmarshal(*bslice, &errMsg); err != nil {
    t.Fatal("Unable to unmarshal bytes slice", testName, err, string(*bslice))
    return
  }
  if errMsg.ErrCode != errCode {
    t.Fatal("[ErrCode] is different than [expected code]", testName, errMsg.ErrCode, errCode, string(*bslice))
    return
  }
}

// SameElements returns True if the two given string slices contain the same
// elements, even in different order.
func SameElements(a0, b0 []string) bool {
  // shallow copy input arrays
  a := append([]string(nil), a0...)
  b := append([]string(nil), b0...)

  if a == nil && b == nil {
    return true
  }
  if a == nil || b == nil {
    return false
  }
  if len(a) != len(b) {
    return false
  }

  sort.Strings(a)
  sort.Strings(b)
  for i := range a {
    if a[i] != b[i] {
      return false
    }
  }
  return true
}
