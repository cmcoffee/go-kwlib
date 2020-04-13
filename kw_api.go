package kwlib

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/cmcoffee/go-iotimeout"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

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
	secrets        kwapi_secrets // Encrypted config options such as signature token, client secret key.
}

// Tests TokenStore, creates one if missing.
func (K *KWAPI) testTokenStore() {
	if K.TokenStore == nil {
		K.TokenStore = KVLiteStore(OpenCache())
	}
}

type kwapi_secrets struct {
	key               []byte
	signature_key     []byte
	client_secret_key []byte
}

// TokenStore interface for saving and retrieving auth tokens.
// Errors should only be underlying issues reading/writing to the store itself.
type TokenStore interface {
	Save(username string, auth *KWAuth) error
	Load(username string) (*KWAuth, error)
	Delete(username string) error
}

type kvLiteStore struct {
	*Database
}

// Wraps KVLite Databse as a auth token store.
func KVLiteStore(input *Database) *kvLiteStore {
	return &kvLiteStore{input}
}

// Save token to TokenStore
func (T kvLiteStore) Save(username string, auth *KWAuth) error {
	T.Database.CryptSet("KWAPI_tokens", username, &auth)
	return nil
}

// Retrieve token from TokenStore
func (T *kvLiteStore) Load(username string) (*KWAuth, error) {
	var auth *KWAuth
	T.Database.Get("KWAPI_tokens", username, &auth)
	return auth, nil
}

// Remove token from TokenStore
func (T *kvLiteStore) Delete(username string) error {
	T.Database.Unset("KWAPI_tokens", username)
	return nil
}

// Encryption function for storing signature and client secrets.
func (k *kwapi_secrets) encrypt(input string) []byte {

	if k.key == nil {
		k.key = RandBytes(32)
	}

	block, err := aes.NewCipher(k.key)
	Critical(err)
	in_bytes := []byte(input)

	buff := make([]byte, len(in_bytes))
	copy(buff, in_bytes)

	cipher.NewCFBEncrypter(block, k.key[0:block.BlockSize()]).XORKeyStream(buff, buff)

	return buff
}

// Retrieves encrypted signature and client secrets.
func (k *kwapi_secrets) decrypt(input []byte) string {
	if k.key == nil {
		return NONE
	}

	output := make([]byte, len(input))

	block, _ := aes.NewCipher(k.key)
	cipher.NewCFBDecrypter(block, k.key[0:block.BlockSize()]).XORKeyStream(output, input)

	return string(output)
}

// APIRequest model
type APIRequest struct {
	APIVer int
	Method string
	Path   string
	Params []interface{}
	Output interface{}
}

// SetPath shortcut.
var SetPath = fmt.Sprintf

// Creates Param for KWAPI post
func SetParams(vars ...interface{}) (output []interface{}) {
	for _, v := range vars {
		output = append(output, v)
	}
	return
}

// Post JSON to KWAPI.
type PostJSON map[string]interface{}

// Form POST to KWAPI.
type PostForm map[string]interface{}

// Add Query params to KWAPI request.
type Query map[string]interface{}

// KWAPI Client
type KWAPIClient struct {
	session *KWSession
	*http.Client
}

func (c *KWAPIClient) Do(req *http.Request) (resp *http.Response, err error) {
	resp, err = c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	err = c.session.respError(resp)
	return
}

// Sets signature key.
func (K *KWAPI) Signature(signature_key string) {
	K.secrets.signature_key = K.secrets.encrypt(signature_key)
}

// Sets client secret key.
func (K *KWAPI) ClientSecret(client_secret_key string) {
	K.secrets.client_secret_key = K.secrets.encrypt(client_secret_key)
}

// kiteworks Auth token.
type KWAuth struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Expires      int64  `json:"expires_in"`
}

// kiteworks Session.
type KWSession struct {
	Username string
	*KWAPI
}

// Wraps a session for specfiied user.
func (K *KWAPI) Session(username string) KWSession {
	return KWSession{username, K}
}

// Prints arrays for string and int arrays, when submitted to Queries or Form post.
func Spanner(input interface{}) string {
	switch v := input.(type) {
	case []string:
		return strings.Join(v, ",")
	case []int:
		var output []string
		for _, i := range v {
			output = append(output, fmt.Sprintf("%v", i))
		}
		return strings.Join(output, ",")
	default:
		return fmt.Sprintf("%v", input)
	}
}

// Decodes JSON response body to provided interface.
func (K *KWAPI) decodeJSON(resp *http.Response, output interface{}) (err error) {

	defer resp.Body.Close()

	var (
		snoop_output map[string]interface{}
		snoop_buffer bytes.Buffer
		body         io.Reader
	)

	resp.Body = iotimeout.NewReadCloser(resp.Body, K.RequestTimeout)

	if K.Snoop {
		if output == nil {
			Stdout("<-- RESPONSE STATUS: %s", resp.Status)
			dec := json.NewDecoder(resp.Body)
			dec.Decode(&snoop_output)
			o, _ := json.MarshalIndent(&snoop_output, "", "  ")
			fmt.Fprintf(os.Stdout, "%s\n", string(o))
			return nil
		} else {
			Stdout("<-- RESPONSE STATUS: %s", resp.Status)
			body = io.TeeReader(resp.Body, &snoop_buffer)
		}
	} else {
		body = resp.Body
	}

	if output == nil {
		return nil
	}

	dec := json.NewDecoder(body)
	err = dec.Decode(output)
	if err == io.EOF {
		return nil
	}

	if err != nil {
		if K.Snoop {
			txt := snoop_buffer.String()
			if err := snoop_request(&snoop_buffer); err != nil {
				Stdout(txt)
			}
			err = fmt.Errorf("I cannot understand what %s is saying: %s", K.Server, err.Error())
			return
		} else {
			err = fmt.Errorf("I cannot understand what %s is saying. (Try running %s --snoop): %s", K.Server, os.Args[0], err.Error())
			return
		}
	}

	if K.Snoop {
		snoop_request(&snoop_buffer)
	}
	return
}

// Provides output of specified request.
func snoop_request(body io.Reader) error {
	var snoop_generic map[string]interface{}
	dec := json.NewDecoder(body)
	if err := dec.Decode(&snoop_generic); err != nil {
		return err
	}
	if snoop_generic != nil {
		for v, _ := range snoop_generic {
			switch v {
			case "refresh_token":
				fallthrough
			case "access_token":
				snoop_generic[v] = "[HIDDEN]"
			}
		}
	}
	o, _ := json.MarshalIndent(&snoop_generic, "", "  ")
	Snoop("%s\n", string(o))
	return nil
}

// kiteworks Client
func (s KWSession) NewClient() *KWAPIClient {
	var transport http.Transport

	// Allows invalid certs if set to "no" in config.
	if !s.VerifySSL {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if s.ProxyURI != NONE {
		proxyURL, err := url.Parse(s.ProxyURI)
		Critical(err)
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	transport.Dial = (&net.Dialer{
		Timeout: s.ConnectTimeout,
	}).Dial

	transport.TLSHandshakeTimeout = s.ConnectTimeout

	return &KWAPIClient{&s, &http.Client{Transport: &transport, Timeout: s.RequestTimeout}}
}

// New kiteworks Request.
func (s KWSession) NewRequest(method, path string, api_ver int) (req *http.Request, err error) {

	// Set API Version
	if api_ver == 0 {
		api_ver = 11
	}

	req, err = http.NewRequest(method, fmt.Sprintf("https://%s%s", s.Server, path), nil)
	if err != nil {
		return nil, err
	}

	req.URL.Host = s.Server
	req.URL.Scheme = "https"
	req.Header.Set("X-Accellion-Version", fmt.Sprintf("%d", api_ver))
	if s.AgentString == NONE {
		s.AgentString = "kwlib/1.0"
	}
	req.Header.Set("User-Agent", s.AgentString)
	req.Header.Set("Referer", "https://"+s.Server+"/")

	if err := s.setToken(req, false); err != nil {
		return nil, err
	}

	return req, nil
}

// kiteworks API Call Wrapper
func (s KWSession) Call(api_req APIRequest) (err error) {

	req, err := s.NewRequest(api_req.Method, api_req.Path, api_req.APIVer)
	if err != nil {
		return err
	}

	if s.Snoop {
		Snoop("\n[kiteworks]: %s", s.Username)
		Snoop("--> METHOD: \"%s\" PATH: \"%s\"", strings.ToUpper(api_req.Method), api_req.Path)
	}

	var body []byte

	for _, in := range api_req.Params {
		switch i := in.(type) {
		case PostForm:
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			p := make(url.Values)
			for k, v := range i {
				p.Add(k, Spanner(v))
				if s.Snoop {
					Snoop("\\-> POST PARAM: \"%s\" VALUE: \"%s\"", k, p[k])
				}
			}
			body = []byte(p.Encode())
		case PostJSON:
			req.Header.Set("Content-Type", "application/json")
			json, err := json.Marshal(i)
			if err != nil {
				return err
			}
			if s.Snoop {
				Snoop("\\-> POST JSON: %s", string(json))
			}
			body = json
		case Query:
			q := req.URL.Query()
			for k, v := range i {
				q.Set(k, Spanner(v))
				if s.Snoop {
					Snoop("\\-> QUERY: %s=%s", k, q[k])
				}
			}
			req.URL.RawQuery = q.Encode()
		default:
			return fmt.Errorf("Unknown request exception.")
		}
	}

	var resp *http.Response

	// Retry calls on failure.
	for i := 0; i <= int(s.Retries); i++ {
		reAuth := func(s *KWSession, req *http.Request, orig_err error) error {
			if s.secrets.signature_key == nil {
				existing, err := s.TokenStore.Load(s.Username)
				if err != nil {
					return err
				}
				if token, err := s.refreshToken(s.Username, existing); err == nil {
					if err := s.TokenStore.Save(s.Username, token); err != nil {
						return err
					}
					if err = s.setToken(req, false); err == nil {
						return nil
					}
				}
				Critical(fmt.Errorf("Token is no longer valid: %s", orig_err.Error()))
				s.TokenStore.Delete(s.Username)
			}
			return s.setToken(req, KWAPIError(err, TOKEN_ERR))
		}

		req.Body = ioutil.NopCloser(bytes.NewReader(body))
		client := s.NewClient()
		resp, err = client.Do(req)
		if err != nil && KWAPIError(err, ERR_INTERNAL_SERVER_ERROR|TOKEN_ERR) {
			Debug("(CALL ERROR) %s -> %s: %s (%d/%d)", s, api_req.Path, err.Error(), i+1, s.Retries+1)
			if err := reAuth(&s, req, err); err != nil {
				return err
			}
			time.Sleep((time.Second * time.Duration(i+1)) * time.Duration(i+1))
			continue
		} else if err != nil {
			break
		}

		err = s.decodeJSON(resp, api_req.Output)
		if err != nil && KWAPIError(err, ERR_INTERNAL_SERVER_ERROR|TOKEN_ERR) {
			Debug("(CALL ERROR) %s -> %s: %s (%d/%d)", s, api_req.Path, err.Error(), i+1, s.Retries+1)
			if err := reAuth(&s, req, err); err != nil {
				return err
			}
			time.Sleep((time.Second * time.Duration(i+1)) * time.Duration(i+1))
			continue
		} else {
			break
		}
	}
	return
}