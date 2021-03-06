package particeps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// AnonFiles is the constant for https://anonfiles.com/
	AnonFiles = iota + 1 // Skip value 0
	// BayFiles is the constant for https://bayfiles.com/
	BayFiles
	// Imgur is the constant for https://imgur.com/
	Imgur
	// Filebin is the constant for https://filebin.net/
	Filebin
	// Imagebin is the constant for https://imagebin.ca/
	Imagebin
)

// CheckFile checks if the filename exists and returns its size in pretty-print form
func CheckFile(filename string) (string, error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "particeps: error: could not find file \"%s\"\n", filename)
		return "", err
	}
	return prettySize(float64(fileInfo.Size())), nil
}

// ImagebinUpload uploads an image to imagebin.ca and returns an UniversalResponse with the upload's data
func ImagebinUpload(filename string) (UniversalResponse, error) {
	// Multi-part Body
	var result UniversalResponse
	result.Status = false
	mpb := bytes.NewBuffer(nil)
	mw := multipart.NewWriter(mpb)

	partWriter, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return result, err
	}

	fileReader, err := os.Open(filename)
	io.Copy(partWriter, fileReader) // TODO: refactor to only call this once when uploading to several providers
	mw.Close()

	resp, err := http.Post("https://imagebin.ca/upload.php", mw.FormDataContentType(), mpb)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	if result.FullURL = getStringAfterWord(string(body), "url:"); result.FullURL == "" {
		return result, nil
	}
	result.Status = true
	return result, nil
}

func getStringAfterWord(value string, word string) string {
	pos := strings.LastIndex(value, word)
	if pos == -1 {
		return ""
	}
	adjustedPos := pos + len(word)
	if adjustedPos >= len(value) {
		return ""
	}
	return value[adjustedPos:len(value)]
}

// Façade function for uploads to Anonfiles and Bayfiles
func uploadFile(filename, destURI string) (UniversalResponse, error) {
	var returnValue UniversalResponse
	returnValue.Status = false

	// Multi-part Body
	mpb := bytes.NewBuffer(nil)
	mw := multipart.NewWriter(mpb)

	partWriter, err := mw.CreateFormFile("file", filename)
	if err != nil {
		log.Fatalln(err) // TODO: return
	}

	fileReader, err := os.Open(filename)
	io.Copy(partWriter, fileReader) // TODO: refactor to only call this once when uploading to several providers
	mw.Close()

	resp, err := http.Post(destURI, mw.FormDataContentType(), mpb) // then we send the multipart body with the file to http.Post
	if err != nil {
		return returnValue, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return returnValue, err
	}

	var successResponse AnonFilesSuccess
	err = json.Unmarshal(body, &successResponse)
	if err != nil {
		return returnValue, err
	}

	returnValue.Status = successResponse.Status
	returnValue.FullURL = successResponse.Data.File.URL.Full
	returnValue.ShortURL = successResponse.Data.File.URL.Short
	return returnValue, nil
}

// // AuthenticateGett reads the user's Gett credentials and saves the account's refresh and access tokens
// func AuthenticateGett(filename string) error {
// 	var email string
// 	fmt.Printf("particeps-auth: please type in your ge.tt's account e-mail: ")
// 	fmt.Scanf("%s\n", &email)
// 	fmt.Printf("\nparticeps-auth: please type in your ge.tt's account password: ")
// 	password, err := terminal.ReadPassword(int(syscall.Stdin))
// 	if err != nil {
// 		return err
// 	}
// 	bodyStr := `{"apikey":"...","email":"` + email + `","password":"` + string(password) + `"}`
// 	body := strings.NewReader(bodyStr)
// 	req, err := http.NewRequest("POST", "https://api.ge.tt/1/users/login", body)
// 	if err != nil {
// 		return res
// 	}
// 	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

// 	resp, err := http.DefaultClient.Do(req)
// 	if err != nil {
// 		// handle err
// 	}
// 	defer resp.Body.Close()

// }

// FilebinUpload uploads the given file to filebin.net and returns a UniversalResponse with status and URL
func FilebinUpload(filename string) (UniversalResponse, error) {
	var returnValue UniversalResponse
	returnValue.Status = false
	f, err := os.Open(filename)
	if err != nil {
		return returnValue, err
	}
	defer f.Close()
	req, err := http.NewRequest("POST", "https://filebin.net", f)
	if err != nil {
		return returnValue, err
	}
	req.Header.Set("Filename", filepath.Base(filename))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return returnValue, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return returnValue, err
	}
	var successResponse FilebinSuccess
	err = json.Unmarshal(body, &successResponse)
	if err != nil {
		return returnValue, err
	}

	if successResponse.Links[1].Href != "" {
		returnValue.FullURL = successResponse.Links[1].Href
		returnValue.Status = true
	}

	return returnValue, nil
}

// BayFilesUpload attemps to upload a file to AnonFiles and returns a success/failure string
func BayFilesUpload(filename string) (UniversalResponse, error) {
	return uploadFile(filename, "https://api.bayfiles.com/upload")
}

// AnonFilesUpload attemps to upload a file to AnonFiles and returns a success/failure string
func AnonFilesUpload(filename string) (UniversalResponse, error) {
	return uploadFile(filename, "https://api.anonfiles.com/upload")
}

func round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

func prettySize(sizeInBytes float64) string {
	suffixes := [4]string{"B", "KB", "MB", "GB"}
	base := math.Log(sizeInBytes) / math.Log(1024)
	size := round(math.Pow(1024, base-math.Floor(base)), .5, 2)
	suffix := suffixes[int(math.Floor(base))]
	return strconv.FormatFloat(size, 'f', -1, 64) + " " + string(suffix)
}