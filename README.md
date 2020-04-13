# kwlib
--
    import "github.com/cmcoffee/go-kwlib"


## Usage

```go
const (
	ERR_AUTH_UNAUTHORIZED = 1 << iota
	ERR_AUTH_PROFILE_CHANGED
	ERR_ACCESS_USER
	ERR_INVALID_GRANT
	ERR_ENTITY_DELETED_PERMANENTLY
	ERR_ENTITY_NOT_FOUND
	ERR_ENTITY_DELETED
	ERR_ENTITY_PARENT_FOLDER_DELETED
	ERR_REQUEST_METHOD_NOT_ALLOWED
	ERR_INTERNAL_SERVER_ERROR
	ERR_ENTITY_EXISTS
	ERR_ENTITY_ROLE_IS_ASSIGNED
	UNAVAILABLE
	SERVICE_UNAVAILABLE
	ERR_ENTITY_NOT_SCANNED
	ERR_ENTITY_PARENT_FOLDER_MEMBER_EXISTS
)
```

```go
const (
	NONE  = ""
	SLASH = string(os.PathSeparator)
)
```

```go
const TOKEN_ERR = ERR_AUTH_PROFILE_CHANGED | ERR_INVALID_GRANT | ERR_AUTH_UNAUTHORIZED
```
Auth token related errors.

```go
var (
	LogFile    = nfo.File
	Log        = nfo.Log
	Fatal      = nfo.Fatal
	Notice     = nfo.Notice
	Flash      = nfo.Flash
	Stdout     = nfo.Stdout
	Warn       = nfo.Warn
	Defer      = nfo.Defer
	Debug      = nfo.Debug
	Snoop      = nfo.Aux
	ForSecret  = ask.ForSecret
	ForInput   = ask.ForInput
	Exit       = nfo.Exit
	PleaseWait = nfo.PleaseWait
)
```
Import from go-nfo.

```go
var ErrNoUploadID = fmt.Errorf("Upload ID not found.")
```

```go
var ErrUploadNoResp = fmt.Errorf("Unexpected empty resposne from server.")
```

```go
var Path = filepath.Clean
```

```go
var SetPath = fmt.Sprintf
```
SetPath shortcut.

#### func  Critical

```go
func Critical(err error)
```
Fatal Error Check

#### func  DateString

```go
func DateString(input time.Time) string
```
Create standard date YY-MM-DD out of time.Time.

#### func  EnableDebug

```go
func EnableDebug()
```
Enable Debug Logging Output

#### func  Err

```go
func Err(input ...interface{})
```

#### func  ErrorCount

```go
func ErrorCount() uint32
```
Returns amount of times Err has been triggered.

#### func  HumanSize

```go
func HumanSize(bytes int64) string
```
Provides human readable file sizes.

#### func  IsKWError

```go
func IsKWError(err error) bool
```
Return true if error was generated by REST call.

#### func  KVLiteStore

```go
func KVLiteStore(input *Database) *kvLiteStore
```
Wraps KVLite Databse as a auth token store.

#### func  KWAPIError

```go
func KWAPIError(err error, input int64) bool
```
Check for specific error code.

#### func  MkDir

```go
func MkDir(name ...string) (err error)
```
Creates folders.

#### func  Quiet

```go
func Quiet()
```
Disables Flash from being displayed.

#### func  RandBytes

```go
func RandBytes(sz int) []byte
```
Generates a random byte slice of length specified.

#### func  ReadKWTime

```go
func ReadKWTime(input string) (time.Time, error)
```
Parse Timestamps from kiteworks

#### func  SetParams

```go
func SetParams(vars ...interface{}) (output []interface{})
```
Creates Param for KWAPI post

#### func  Spanner

```go
func Spanner(input interface{}) string
```
Prints arrays for string and int arrays, when submitted to Queries or Form post.

#### func  WriteKWTime

```go
func WriteKWTime(input time.Time) string
```
Write timestamps for kiteworks.

#### type APIRequest

```go
type APIRequest struct {
	APIVer int
	Method string
	Path   string
	Params []interface{}
	Output interface{}
}
```

APIRequest model

#### type BitFlag

```go
type BitFlag int64
```

Atomic BitFlag

#### func (*BitFlag) Has

```go
func (B *BitFlag) Has(flag int) bool
```

#### func (*BitFlag) Set

```go
func (B *BitFlag) Set(flag int)
```
Set BitFlag

#### func (*BitFlag) Unset

```go
func (B *BitFlag) Unset(flag int)
```
Unset BitFlag

#### type Database

```go
type Database struct {
}
```

Wrapper around go-kvlite.

#### func  OpenCache

```go
func OpenCache() *Database
```
Open a memory-only go-kvlite store.

#### func  OpenDatabase

```go
func OpenDatabase(file string, padlock ...[]byte) (*Database, error)
```
Opens go-kvlite sqlite database.

#### func (Database) Close

```go
func (d Database) Close()
```
Closes go-kvlite database.

#### func (Database) CryptSet

```go
func (d Database) CryptSet(table string, key, value interface{})
```
Encrypt value to go-kvlie, fatal on error.

#### func (Database) Get

```go
func (d Database) Get(table string, key, output interface{}) bool
```
Retrieve value from go-kvlite.

#### func (Database) ListKeys

```go
func (d Database) ListKeys(table string) []string
```
List keys in go-kvlite.

#### func (Database) ListNKeys

```go
func (d Database) ListNKeys(table string) []int
```
List numeric keys in go-kvlite.

#### func (Database) Set

```go
func (d Database) Set(table string, key, value interface{})
```
Save value to go-kvlite.

#### func (Database) Truncate

```go
func (d Database) Truncate(table string)
```
DB Wrappers to perform fatal error checks on each call.

#### func (Database) Unset

```go
func (d Database) Unset(table string, key interface{})
```
Delete value from go-kvlite.

#### type Error

```go
type Error string
```

Error handler for const errors.

#### func (Error) Error

```go
func (e Error) Error() string
```

#### type KWAPI

```go
type KWAPI struct {
	Server         string        // kiteworks host name.
	ApplicationID  string        // Application ID set for kiteworks custom app.
	RedirectURI    string        // Redirect URI for kiteworks custom app.
	AgentString    string        // Agent-String header for calls to kiteworks.
	VerifySSL      bool          // Verify certificate for connections.
	ProxyURI       string        // Proxy for outgoing https requests.
	Snoop          bool          // Flag to snoop API calls
	RequestTimeout time.Duration // Timeout for request to be answered from kiteworks server.
	ConnectTimeout time.Duration // Timeout for TLS connection to kiteworks server.
	MaxChunkSize   int64         // Max Upload Chunksize in bytes, min = 1M, max = 68M
	Retries        uint          // Max retries on a failed call
	TokenStore     TokenStore    // TokenStore for reading and writing auth tokens securely.
}
```


#### func (*KWAPI) Authenticate

```go
func (K *KWAPI) Authenticate(username string) (*KWSession, error)
```
Set User Credentials for kw_api.

#### func (*KWAPI) ClientSecret

```go
func (K *KWAPI) ClientSecret(client_secret_key string)
```
Sets client secret key.

#### func (*KWAPI) Session

```go
func (K *KWAPI) Session(username string) KWSession
```
Wraps a session for specfiied user.

#### func (*KWAPI) Signature

```go
func (K *KWAPI) Signature(signature_key string)
```
Sets signature key.

#### type KWAPIClient

```go
type KWAPIClient struct {
	*http.Client
}
```

KWAPI Client

#### func (*KWAPIClient) Do

```go
func (c *KWAPIClient) Do(req *http.Request) (resp *http.Response, err error)
```

#### type KWAuth

```go
type KWAuth struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Expires      int64  `json:"expires_in"`
}
```

kiteworks Auth token.

#### type KWError

```go
type KWError struct {
}
```

Specific kiteworks error object.

#### func  NewKWError

```go
func NewKWError() *KWError
```
Create a new REST error.

#### func (*KWError) AddError

```go
func (e *KWError) AddError(code, message string)
```
Add a kiteworks error to APIError

#### func (KWError) Error

```go
func (e KWError) Error() string
```
Returns Error String.

#### type KWSession

```go
type KWSession struct {
	Username string
	*KWAPI
}
```

kiteworks Session.

#### func (KWSession) Call

```go
func (s KWSession) Call(api_req APIRequest) (err error)
```
kiteworks API Call Wrapper

#### func (KWSession) Download

```go
func (s KWSession) Download(file_id int) (io.ReadSeeker, error)
```
Downloads a file to a specific path

#### func (*KWSession) ExtDownload

```go
func (S *KWSession) ExtDownload(req *http.Request) io.ReadSeeker
```
Perform External Download from a remote request.

#### func (KWSession) NewClient

```go
func (s KWSession) NewClient() *KWAPIClient
```
kiteworks Client

#### func (KWSession) NewRequest

```go
func (s KWSession) NewRequest(method, path string, api_ver int) (req *http.Request, err error)
```
New kiteworks Request.

#### func (*KWSession) NewUpload

```go
func (S *KWSession) NewUpload(folder_id int, filename string, file_size int64) (int, error)
```
Creates a new upload for a folder.

#### func (*KWSession) NewVersion

```go
func (S *KWSession) NewVersion(file_id int, filename string, file_size int64) (int, error)
```
Create a new file version for an existing file.

#### func (KWSession) Upload

```go
func (s KWSession) Upload(filename string, upload_id int, source io.ReadSeeker) (int, error)
```
Uploads file from specific local path, uploads in chunks, allows resume.

#### type PostForm

```go
type PostForm map[string]interface{}
```

Form POST to KWAPI.

#### type PostJSON

```go
type PostJSON map[string]interface{}
```

Post JSON to KWAPI.

#### type Query

```go
type Query map[string]interface{}
```

Add Query params to KWAPI request.

#### type TMonitor

```go
type TMonitor struct {
}
```

Transfer Monitor

#### func  TransferMonitor

```go
func TransferMonitor(name string, total_sz int64) *TMonitor
```
Add Transfer to transferDisplay. Parameters are "name" displayed for file
transfer, "limit_sz" for when to pause transfer (aka between calls/chunks), and
"total_sz" the total size of the transfer.

#### func (*TMonitor) Close

```go
func (tm *TMonitor) Close()
```
Removes TMonitor from transferDisplay.

#### func (*TMonitor) Offset

```go
func (t *TMonitor) Offset(current_sz int64)
```
Sets the offset to the size already tansfered.

#### func (*TMonitor) RecordTransfer

```go
func (t *TMonitor) RecordTransfer(current_sz int)
```
Update transfered bytes

#### type TokenStore

```go
type TokenStore interface {
	Save(username string, auth *KWAuth) error
	Load(username string) (*KWAuth, error)
	Delete(username string) error
}
```

TokenStore interface for saving and retrieving auth tokens. Errors should only
be underlying issues reading/writing to the store itself.