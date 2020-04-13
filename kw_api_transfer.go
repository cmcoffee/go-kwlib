package kwlib

import (
	"bytes"
	"fmt"
	"github.com/cmcoffee/go-iotimeout"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Max/Min chunk size for kiteworks
const (
	kw_chunk_size_max = 68157440
	kw_chunk_size_min = 1048576
)

var ErrNoUploadID = fmt.Errorf("Upload ID not found.")
var ErrUploadNoResp = fmt.Errorf("Unexpected empty resposne from server.")

// Returns chunk_size, total number of chunks and last chunk size.
func (K *KWAPI) getChunkInfo(total_size int64) (total_chunks int64) {
	chunk_size := K.MaxChunkSize

	if chunk_size == 0 || chunk_size > kw_chunk_size_max {
		chunk_size = kw_chunk_size_max
	}

	if chunk_size <= kw_chunk_size_min {
		chunk_size = kw_chunk_size_min
	}

	if total_size <= chunk_size {
		return 1
	}

	for total_size%chunk_size > 0 {
		chunk_size--
	}

	return total_size / chunk_size
}

const (
	wd_started = 1 << iota
)

// Webdownloader for external sources
type web_downloader struct {
	flag            BitFlag
	err             error
	req             *http.Request
	client          *http.Client
	resp            *http.Response
	offset          int64
	request_timeout time.Duration
}

func (W *web_downloader) Read(p []byte) (n int, err error) {
	if !W.flag.Has(wd_started) {
		if W.req == nil || W.client == nil {
			return 0, fmt.Errorf("Webdownloader not initialized.")
		} else {
			W.flag.Set(wd_started)
			W.client.Timeout = 0
			W.resp, err = W.client.Do(W.req)
			if err != nil {
				return 0, err
			}
			if W.resp.StatusCode < 200 || W.resp.StatusCode >= 300 {
				return 0, fmt.Errorf("GET %s: %s", W.req.URL, W.resp.Status)
			}
			if W.offset > 0 {
				content_range := strings.Split(strings.TrimPrefix(W.resp.Header.Get("Content-Range"), "bytes"), "-")
				if len(content_range) > 1 {
					if strings.TrimSpace(content_range[0]) != strconv.FormatInt(W.offset, 10) {
						return 0, fmt.Errorf("Requested byte %v, got %v instead.", W.offset, content_range[0])
					}
				}
			}
			W.resp.Body = iotimeout.NewReadCloser(W.resp.Body, W.request_timeout)
		}
	}
	n, err = W.resp.Body.Read(p)
	if err != nil {
		defer W.resp.Body.Close()
	}
	return
}

func (W *web_downloader) Seek(offset int64, whence int) (int64, error) {
	if offset < 0 {
		return 0, fmt.Errorf("Can't read before the start of the file.")
	}
	W.req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	W.offset = offset
	return offset, nil
}

// Perform External Download from a remote request.
func (S *KWSession) ExtDownload(req *http.Request) io.ReadSeeker {
	req.Header.Set("Content-Type", "application/octet-stream")
	if S.AgentString == "" {
		req.Header.Set("User-Agent", "kwlib/1.0")
	} else {
		req.Header.Set("User-Agent", S.AgentString)
	}

	client := S.NewClient()
	return &web_downloader{
		req:             req,
		client:          client.Client,
		request_timeout: S.RequestTimeout,
	}
}

// Creates a new upload for a folder.
func (S *KWSession) NewUpload(folder_id int, filename string, file_size int64) (int, error) {
	var upload struct {
		ID int `json:"id"`
	}

	if err := S.Call(APIRequest{
		APIVer: 5,
		Method: "POST",
		Path:   SetPath("/rest/folders/%d/actions/initiateUpload", folder_id),
		Params: SetParams(PostJSON{"filename": filename, "totalSize": file_size, "totalChunks": S.getChunkInfo(file_size)}, Query{"returnEntity": true}),
		Output: &upload,
	}); err != nil {
		return -1, err
	}
	return upload.ID, nil
}

// Create a new file version for an existing file.
func (S *KWSession) NewVersion(file_id int, filename string, file_size int64) (int, error) {
	var upload struct {
		ID int `json:"id"`
	}

	if err := S.Call(APIRequest{
		Method: "POST",
		Path:   SetPath("/rest/files/%d/actions/initiateUpload", file_id),
		Params: SetParams(PostJSON{"filename": filename, "totalSize": file_size, "totalChunks": S.getChunkInfo(file_size)}, Query{"returnEntity": true}),
		Output: &upload,
	}); err != nil {
		return -1, err
	}
	return upload.ID, nil
}

// Multipart filestreamer
type streamReadCloser struct {
	chunkSize int64
	size      int64
	r_buff    []byte
	w_buff    *bytes.Buffer
	source    io.Reader
	eof       bool
	f_writer  io.Writer
	tm        *TMonitor
	*multipart.Writer
}

// Read function fro streamReadCloser, reads triggers a read from source->writes to bytes buffer via multipart writer->reads from bytes buffer.
func (s *streamReadCloser) Read(p []byte) (n int, err error) {
	buf_len := s.w_buff.Len()

	if buf_len > 0 {
		n, err = s.w_buff.Read(p)
		return
	}

	// Clear our output buffer.
	s.w_buff.Truncate(0)

	if s.eof {
		s.Close()
		return 0, io.EOF
	}

	if !s.eof && s.chunkSize-s.size <= 4096 {
		s.r_buff = s.r_buff[0 : s.chunkSize-s.size]
		s.eof = true
	}

	n, err = s.source.Read(s.r_buff)
	if err != nil && err == io.EOF {
		s.eof = true
	} else if err != nil {
		return -1, err
	}

	s.size = s.size + int64(n)
	if n > 0 {
		n, err = s.f_writer.Write(s.r_buff[0:n])
		if err != nil {
			return -1, err
		}
		s.tm.RecordTransfer(n)
		if s.eof {
			s.Close()
		}
		for i := 0; i < len(s.r_buff); i++ {
			s.r_buff[i] = 0
		}
	}
	n, err = s.w_buff.Read(p)
	return
}

// Uploads file from specific local path, uploads in chunks, allows resume.
func (s KWSession) Upload(filename string, upload_id int, source io.ReadSeeker) (int, error) {
	type upload_data struct {
		ID             int    `json:"id"`
		TotalSize      int64  `json:"totalSize"`
		TotalChunks    int64  `json:"totalChunks"`
		UploadedSize   int64  `json:"uploadedSize"`
		UploadedChunks int64  `json:"uploadedChunks"`
		Finished       bool   `json:"finished"`
		URI            string `json:"uri"`
	}

	var upload struct {
		Data []upload_data `json:"data"`
	}

	err := s.Call(APIRequest{
		Method: "GET",
		Path:   "/rest/uploads",
		Params: SetParams(Query{"locate_id": upload_id, "limit": 1, "with": "(id,totalSize,totalChunks,uploadedChunks,finished,uploadedSize)"}),
		Output: &upload,
	})
	if err != nil {
		return -1, err
	}

	var upload_record upload_data

	if upload.Data != nil && len(upload.Data) > 0 {
		upload_record = upload.Data[0]
	}

	if upload_id != upload_record.ID {
		return -1, ErrNoUploadID
	}

	total_bytes := upload_record.TotalSize

	ChunkSize := upload_record.TotalSize / upload_record.TotalChunks
	ChunkIndex := upload_record.UploadedChunks

	if ChunkIndex > 0 {
		if upload_record.UploadedSize > 0 && upload_record.UploadedChunks > 0 {
			if _, err = source.Seek(ChunkSize*ChunkIndex, 0); err != nil {
				return -1, err
			}
		}
	}

	transfered_bytes := upload_record.UploadedSize

	w_buff := new(bytes.Buffer)

	tm := TransferMonitor(filename, total_bytes)
	defer tm.Close()

	tm.Offset(transfered_bytes)

	var resp_data struct {
		ID int `json:"id"`
	}

	for transfered_bytes < total_bytes || total_bytes == 0 {
		w_buff.Reset()

		req, err := s.NewRequest("POST", fmt.Sprintf("/%s", upload_record.URI), 7)
		if err != nil {
			return -1, err
		}

		if s.Snoop {
			Snoop("\n[kiteworks]: %s", s.Username)
			Snoop("--> METHOD: \"POST\" PATH: \"%v\" (CHUNK %d OF %d)\n", req.URL.Path, ChunkIndex+1, upload_record.TotalChunks)
		}

		w := multipart.NewWriter(w_buff)

		req.Header.Set("Content-Type", "multipart/form-data; boundary="+w.Boundary())

		if ChunkIndex == upload_record.TotalChunks-1 {
			q := req.URL.Query()
			q.Set("returnEntity", "true")
			q.Set("mode", "full")
			if s.Snoop {
				for k, v := range q {
					Snoop("\\-> QUERY: %s VALUE: %s", k, v)
				}
			}
			req.URL.RawQuery = q.Encode()
			ChunkSize = total_bytes - transfered_bytes
		}

		err = w.WriteField("compressionMode", "NORMAL")
		if err != nil {
			return -1, err
		}

		err = w.WriteField("index", fmt.Sprintf("%d", ChunkIndex+1))
		if err != nil {
			return -1, err
		}

		err = w.WriteField("compressionSize", fmt.Sprintf("%d", ChunkSize))
		if err != nil {
			return -1, err
		}

		err = w.WriteField("originalSize", fmt.Sprintf("%d", ChunkSize))
		if err != nil {
			return -1, err
		}

		f_writer, err := w.CreateFormFile("content", filename)
		if err != nil {
			return -1, err
		}

		if s.Snoop {
			Snoop(w_buff.String())
		}

		post := &streamReadCloser{
			ChunkSize,
			0,
			make([]byte, 4096),
			w_buff,
			iotimeout.NewReader(source, s.RequestTimeout),
			false,
			f_writer,
			tm,
			w,
		}

		req.Body = post
		client := s.NewClient()
		client.Timeout = 0

		resp, err := client.Do(req)
		if err != nil {
			return -1, err
		}

		if err := s.decodeJSON(resp, &resp_data); err != nil {
			return -1, err
		}

		ChunkIndex++
		transfered_bytes = transfered_bytes + ChunkSize
		if total_bytes == 0 {
			break
		}
	}

	if resp_data.ID == 0 {
		return -1, ErrUploadNoResp
	}

	return resp_data.ID, nil
}

// Pass-thru reader for reporting.
type transfer_reader struct {
	exit int32
	src  io.ReadSeeker
	tm   *TMonitor
}

func (T *transfer_reader) Seek(offset int64, whence int) (int64, error) {
	T.tm.Offset(offset)
	return T.src.Seek(offset, whence)
}

func (T *transfer_reader) Read(p []byte) (n int, err error) {
	n, err = T.src.Read(p)
	T.tm.RecordTransfer(n)
	if err != nil {
		defer T.tm.Close()
	}
	return
}

func (T *transfer_reader) Close() (err error) {
	T.tm.Close()
	return nil
}

// Downloads a file to a specific path
func (s KWSession) Download(file_id int) (io.ReadSeeker, error) {
	var file_info struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}

	if err := s.Call(APIRequest{
		Method: "GET",
		Path:   SetPath("/rest/files/%d", file_id),
		Output: &file_info,
	}); err != nil {
		return nil, err
	}

	filename := file_info.Name
	total_bytes := file_info.Size

	req, err := s.NewRequest("GET", SetPath("/rest/files/%d/content", file_id), 7)
	if err != nil {
		return nil, err
	}

	dl := s.ExtDownload(req)

	transfer := &transfer_reader{
		0,
		dl,
		TransferMonitor(filename, total_bytes),
	}

	return transfer, nil
}