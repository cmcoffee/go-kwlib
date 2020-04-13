package kwlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cmcoffee/go-iotimeout"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

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

// Auth token related errors.
const TOKEN_ERR = ERR_AUTH_PROFILE_CHANGED | ERR_INVALID_GRANT | ERR_AUTH_UNAUTHORIZED

// Specific kiteworks error object.
type KWError struct {
	flag    int64
	message []string
}

// Add a kiteworks error to APIError
func (e *KWError) AddError(code, message string) {
	code = strings.ToUpper(code)

	switch code {
	case "ERR_ENTITY_PARENT_FOLDER_MEMBER_EXISTS":
		e.flag |= ERR_ENTITY_PARENT_FOLDER_MEMBER_EXISTS
	case "ERR_ENTITY_NOT_SCANNED":
		e.flag |= ERR_ENTITY_NOT_SCANNED
	case "ERR_ENTITY_ROLE_IS_ASSIGNED":
		e.flag |= ERR_ENTITY_ROLE_IS_ASSIGNED
	case "ERR_ENTITY_EXISTS":
		e.flag |= ERR_ENTITY_EXISTS
	case "ERR_AUTH_UNAUTHORIZED":
		e.flag |= ERR_AUTH_UNAUTHORIZED
	case "unauthorized_client":
		e.flag |= ERR_AUTH_UNAUTHORIZED
	case "ERR_AUTH_PROFILE_CHANGED":
		e.flag |= ERR_AUTH_PROFILE_CHANGED
	case "ERR_ACCESS_USER":
		e.flag |= ERR_ACCESS_USER
	case "INVALID_GRANT":
		e.flag |= ERR_INVALID_GRANT
	case "ERR_ENTITY_DELETED_PERMANENTLY":
		e.flag |= ERR_ENTITY_DELETED_PERMANENTLY
	case "ERR_ENTITY_DELETED":
		e.flag |= ERR_ENTITY_DELETED
	case "ERR_ENTITY_NOT_FOUND":
		e.flag |= ERR_ENTITY_NOT_FOUND
	case "ERR_ENTITY_PARENT_FOLDER_DELETED":
		e.flag |= ERR_ENTITY_PARENT_FOLDER_DELETED
	case "ERR_REQUEST_METHOD_NOT_ALLOWED":
		e.flag |= ERR_REQUEST_METHOD_NOT_ALLOWED
	case "UNAVAILABLE":
		e.flag |= UNAVAILABLE
	default:
		if strings.Contains(code, "ERR_INTERNAL_") {
			e.flag |= ERR_INTERNAL_SERVER_ERROR
		}
	}
	e.message = append(e.message, fmt.Sprintf("%s. (kiteworks:%s)", message, code))
}

// Returns Error String.
func (e KWError) Error() string {
	str := make([]string, 0)
	e_len := len(e.message)
	for i := 0; i < e_len; i++ {
		if e_len == 1 {
			return e.message[i]
		} else {
			str = append(str, fmt.Sprintf("[%d] %s\n", i, e.message[i]))
		}
	}
	return strings.Join(str, "\n")
}

// Check for specific error code.
func KWAPIError(err error, input int64) bool {
	if e, ok := err.(*KWError); !ok {
		return false
	} else {
		if e.flag&input != 0 {
			return true
		}
	}
	return false
}

// Return true if error was generated by REST call.
func IsKWError(err error) bool {
	if _, ok := err.(*KWError); ok {
		return true
	}
	return false
}

// Create a new REST error.
func NewKWError() *KWError {
	e := new(KWError)
	e.message = make([]string, 0)
	return e
}

// convert responses from kiteworks APIs to errors to return to callers.
func (K *KWAPI) respError(resp *http.Response) (err error) {
	if resp == nil {
		return
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var (
		snoop_buffer bytes.Buffer
		body         io.Reader
	)

	resp.Body = iotimeout.NewReadCloser(resp.Body, K.RequestTimeout)

	if K.Snoop {
		Snoop("<-- RESPONSE STATUS: %s", resp.Status)
		body = io.TeeReader(resp.Body, &snoop_buffer)
	} else {
		body = resp.Body
	}

	// kiteworks API Error
	type KiteErr struct {
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
		Errors    []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}

	output, err := ioutil.ReadAll(body)

	if K.Snoop {
		snoop_request(&snoop_buffer)
	}

	if err != nil {
		return err
	}

	var kite_err *KiteErr
	json.Unmarshal(output, &kite_err)
	if kite_err != nil {
		e := NewKWError()
		for _, v := range kite_err.Errors {
			e.AddError(v.Code, v.Message)
		}
		if kite_err.ErrorDesc != NONE {
			e.AddError(kite_err.Error, kite_err.ErrorDesc)
		}
		return e
	}

	if resp.Status == "401 Unathorized" {
		e := NewKWError()
		e.AddError("ERR_AUTH_UNAUTHORIZED", "Unathorized Access Token")
		return e
	}

	return fmt.Errorf("%s says \"%s.\"", resp.Request.Host, resp.Status)
}
