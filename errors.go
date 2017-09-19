package igngo

import (
  "net/http"
)

///////////////////////
// Database error codes
///////////////////////

// ErrorNoDatabase is triggered when the database connection is unavailable
const ErrorNoDatabase      = 1000
// ErrorDbDelete is triggered when the database was unable to delete a resource
const ErrorDbDelete        = 1001
// ErrorDbSave is triggered when the database was unable to save a resource
const ErrorDbSave          = 1002
// ErrorIDNotFound is triggered when a resource with the specified id is not
// found in the database
const ErrorIDNotFound      = 1003
// ErrorNameNotFound is triggered when a resource with the specified name is not
// found in the database
const ErrorNameNotFound    = 1004

// ErrorFileNotFound is triggered when a model's file with the specified name is not
// found
const ErrorFileNotFound    = 1005

///////////////////
// JSON error codes
///////////////////

// ErrorMarshalJSON is triggered if there is an error marshalling data into JSON
const ErrorMarshalJSON     = 2000
// ErrorUnmarshalJSON is triggered if there is an error unmarshalling JSON
const ErrorUnmarshalJSON   = 2001

///////////////////
// Protobuf error codes
///////////////////

// ErrorMarshalProto is triggered if there is an error marshalling data into protobuf
const ErrorMarshalProto     = 2500

//////////////////////
// Request error codes
//////////////////////

// ErrorIDNotInRequest is triggered when a id is not found in the request
const ErrorIDNotInRequest  = 3000
// ErrorIDWrongFormat is triggered when an id is not in a valid format
const ErrorIDWrongFormat   = 3001
// ErrorNameWrongFormat is triggered when a name is not in a valid format
const ErrorNameWrongFormat   = 3002
// ErrorPayloadEmpty is triggered when payload is expected but is not found in
// the request
const ErrorPayloadEmpty      = 3003
// ErrorForm is triggered when an expected field is missing in a multipart
// form request.
const ErrorForm              = 3004
// ErrorUnexpectedID is triggered when the id of a file attached in a
// request is not expected. E.g.: When the attached world file does not end in
// ".world" during a world creation request.
const ErrorUnexpectedID    = 3005
// ErrorUnknownSuffix is triggered when a suffix for content negotiation is not
// recognized.
const ErrorUnknownSuffix     = 3006
// ErrorUserNotInRequest is triggered when the user/team is not found in
// the request.
const ErrorUserNotInRequest = 3007
// ErrorUserUnknown is triggered when the user/team does not exist on the
// server
const ErrorUserUnknown      = 3008
// ErrorMissingField is triggered when the JSON contained in a request does
// not contain one or more required fields
const ErrorMissingField      = 3009
// ErrorOwnerNotInRequest is triggered when an owner is not found in the request
const ErrorOwnerNotInRequest  = 3010
// ErrorModelNotInRequest is triggered when a model is not found in the request
const ErrorModelNotInRequest  = 3011

////////////////////////////
// Authorization error codes
////////////////////////////

// ErrorAuthNoUser is triggered when there's no user in the database with the
// claimed user ID.
const ErrorAuthNoUser      = 4000
// ErrorAuthJWTInvalid is triggered when is not possible to get a user ID
// from the JWT in the request
const ErrorAuthJWTInvalid  = 4001
// ErrorUnauthorized is triggered when a user is not authorized to perform a
// given action.
const ErrorUnauthorized    = 4002

////////////////////
// Other error codes
////////////////////

// ErrorZipNotAvailable is triggered when the server does not have a zip file
// for the requested resource
const ErrorZipNotAvailable     = 100000
// ErrorResourceExists is triggered when the server cannot create a new resource
// because the requested id already exists. E.g.: When the creation of a new
// model is requested but the server already has a model with the same id.
const ErrorResourceExists      = 100001
// ErrorCreatingDir is triggered when the server was unable to create a new
// directory for a resource (no space on device or a temporary server problem).
const ErrorCreatingDir         = 100002
// ErrorCreatingRepo is triggered when the server was unable to create a new
// repository for a resource (no space on device or a temporary server problem).
const ErrorCreatingRepo        = 100003
// ErrorCreatingFile is triggered when the server was unable to create a new
// file for a resource (no space on device or a temporary server problem).
const ErrorCreatingFile        = 100004
// ErrorUnzipping is triggered when the server was unable to unzip a zipped file
const ErrorUnzipping           = 100005
// ErrorNonExistentResource is triggered when the server was unable to find a
// resource.
const ErrorNonExistentResource = 100006
// ErrorRepo is triggered when the server was unable to handle repo command.
const ErrorRepo                = 100007
// ErrorRemovingDir is triggered when the server was unable to remove a 
// directory.
const ErrorRemovingDir         = 100008
// ErrorFileTree is triggered when there was a problem accessing the model's
// files.
const ErrorFileTree            = 100009

// ErrMsg is serialized as JSON, and returned if the request does not succeed
// TODO: consider making ErrMsg an 'error'
type ErrMsg struct {
  // Internal error code.
  ErrCode    int    `json:"errcode"`
  // HTTP status code.
  StatusCode int    `json:"-"`
  // Error message.
  Msg        string `json:"msg"`

  BaseError   error `json:"-"`
}

// NewErrorMessage is a convenience function that receives an error code
// and returns a pointer to an ErrMsg.
func NewErrorMessage(err int64) (*ErrMsg) {
  em := ErrorMessage(err)
  return &em
}

// NewErrorMessageWithBase receives an error code and a root error
// and returns a pointer to an ErrMsg.
func NewErrorMessageWithBase(err int64, base error) (*ErrMsg) {
  em := NewErrorMessage(err)
  em.BaseError = base
  return em
}

// ErrorMessageOK creates an ErrMsg initialized with OK (default) values.
func ErrorMessageOK() (ErrMsg) {
  return ErrMsg{ErrCode: 0, StatusCode: http.StatusOK, Msg: ""}
}

// ErrorMessage receives an error code and generate an error message response
func ErrorMessage(err int64) (ErrMsg) {

  em := ErrorMessageOK()

  switch (err) {
    case ErrorNoDatabase:
      em.Msg = "Unable to connect to the database"
      em.ErrCode = ErrorNoDatabase
      em.StatusCode = http.StatusServiceUnavailable
    case ErrorDbDelete:
      em.Msg = "Unable to remove resource from the database"
      em.ErrCode = ErrorDbDelete
      em.StatusCode = http.StatusInternalServerError
    case ErrorDbSave:
      em.Msg = "Unable to save resource into the database"
      em.ErrCode = ErrorDbSave
      em.StatusCode = http.StatusInternalServerError
    case ErrorIDNotFound:
      em.Msg = "Requested id not found on server"
      em.ErrCode = ErrorIDNotFound
      em.StatusCode = http.StatusNotFound
    case ErrorNameNotFound:
      em.Msg = "Requested name not found on server"
      em.ErrCode = ErrorNameNotFound
      em.StatusCode = http.StatusNotFound
    case ErrorFileNotFound:
      em.Msg = "Requested file not found on server"
      em.ErrCode = ErrorFileNotFound
      em.StatusCode = http.StatusNotFound
    case ErrorMarshalJSON:
      em.Msg = "Unable to marshal the response into a JSON"
      em.ErrCode = ErrorMarshalJSON
      em.StatusCode = http.StatusInternalServerError
     case ErrorUnmarshalJSON:
      em.Msg = "Unable to decode JSON payload included in the request"
      em.ErrCode = ErrorUnmarshalJSON
      em.StatusCode = http.StatusBadRequest
    case ErrorMarshalProto:
      em.Msg = "Unable to marshal the response into a protobuf"
      em.ErrCode = ErrorMarshalProto
      em.StatusCode = http.StatusInternalServerError
    case ErrorIDNotInRequest:
      em.Msg = "ID not present in request"
      em.ErrCode = ErrorIDNotInRequest
      em.StatusCode = http.StatusBadRequest
    case ErrorOwnerNotInRequest:
      em.Msg = "Owner name not present in request"
      em.ErrCode = ErrorOwnerNotInRequest
      em.StatusCode = http.StatusBadRequest
    case ErrorModelNotInRequest:
      em.Msg = "Model name not present in request"
      em.ErrCode = ErrorModelNotInRequest
      em.StatusCode = http.StatusBadRequest
    case ErrorIDWrongFormat:
      em.Msg = "ID in request is in an invalid format"
      em.ErrCode = ErrorIDWrongFormat
      em.StatusCode = http.StatusBadRequest
    case ErrorNameWrongFormat:
      em.Msg = "Name in request is in an invalid format"
      em.ErrCode = ErrorNameWrongFormat
      em.StatusCode = http.StatusBadRequest
    case ErrorPayloadEmpty:
      em.Msg = "Payload empty in the request"
      em.ErrCode = ErrorPayloadEmpty
      em.StatusCode = http.StatusBadRequest
    case ErrorForm:
      em.Msg = "Missing field in the multipart form"
      em.ErrCode = ErrorForm
      em.StatusCode = http.StatusBadRequest
     case ErrorUnexpectedID:
      em.Msg = "Unexpected id included in your request"
      em.ErrCode = ErrorUnexpectedID
      em.StatusCode = http.StatusBadRequest
     case ErrorUnknownSuffix:
      em.Msg = "Unknown suffix requested"
      em.ErrCode = ErrorUnknownSuffix
      em.StatusCode = http.StatusBadRequest
    case ErrorUserNotInRequest:
      em.Msg = "User or team not present in the request"
      em.ErrCode = ErrorUserNotInRequest
      em.StatusCode = http.StatusBadRequest
    case ErrorUserUnknown:
      em.Msg = "Provided user or team does not exist on the server"
      em.ErrCode = ErrorUserUnknown
      em.StatusCode = http.StatusBadRequest
    case ErrorMissingField:
      em.Msg = "One or more required fields are missing"
      em.ErrCode = ErrorMissingField
      em.StatusCode = http.StatusBadRequest
    case ErrorAuthNoUser:
      em.Msg = "No user in server with the claimed identity"
      em.ErrCode = ErrorAuthNoUser
      em.StatusCode = http.StatusForbidden
    case ErrorAuthJWTInvalid:
      em.Msg = "Unable to process user ID from the JWT included in request"
      em.ErrCode = ErrorAuthJWTInvalid
      em.StatusCode = http.StatusForbidden
    case ErrorUnauthorized:
      em.Msg = "Unauthorized request"
      em.ErrCode = ErrorAuthJWTInvalid
      em.StatusCode = http.StatusUnauthorized
    case ErrorZipNotAvailable:
      em.Msg = "Zip file not available for this resource"
      em.ErrCode = ErrorZipNotAvailable
      em.StatusCode = http.StatusServiceUnavailable
    case ErrorResourceExists:
      em.Msg = "A resource with the same id already exists"
      em.ErrCode = ErrorResourceExists
      em.StatusCode = http.StatusConflict
    case ErrorCreatingDir:
      em.Msg = "Unable to create a new directory for the resource"
      em.ErrCode = ErrorCreatingDir
      em.StatusCode = http.StatusInternalServerError
    case ErrorCreatingRepo:
      em.Msg = "Unable to create a new repository for the resource"
      em.ErrCode = ErrorCreatingRepo
      em.StatusCode = http.StatusInternalServerError
    case ErrorCreatingFile:
      em.Msg = "Unable to create a new file for the resource"
      em.ErrCode = ErrorCreatingFile
      em.StatusCode = http.StatusInternalServerError
    case ErrorUnzipping:
      em.Msg = "Unable to unzip a file"
      em.ErrCode = ErrorUnzipping
      em.StatusCode = http.StatusBadRequest
    case ErrorNonExistentResource:
      em.Msg = "Unable to find the requested resource"
      em.ErrCode = ErrorNonExistentResource
      em.StatusCode = http.StatusServiceUnavailable
    case ErrorRepo:
      em.Msg = "Unable to process repository command"
      em.ErrCode = ErrorRepo
      em.StatusCode = http.StatusServiceUnavailable
    case ErrorRemovingDir:
      em.Msg = "Unable to remove a resource directory"
      em.ErrCode = ErrorRemovingDir
      em.StatusCode = http.StatusInternalServerError
    case ErrorFileTree:
      em.Msg = "Unable to get files from model"
      em.ErrCode = ErrorFileTree
      em.StatusCode = http.StatusInternalServerError
  }

  return em
}
