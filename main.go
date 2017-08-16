package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// Directory is the root directory for accessing files
var Directory string

var https = flag.Bool("https", false, "enable https")
var cert = flag.String("cert", "server.crt", "https cert file")
var key = flag.String("key", "server.key", "https key file")
var port = flag.String("p", "8080", "Port Number")
var dir = flag.String("d", "./", "Root Directory")

func main() {
	var err error

	flag.Parse()

	Directory = *dir

	fmt.Println("Serving HTTP on 0.0.0.0 port", *port, "dir", *dir, "...")

	http.HandleFunc("/", Handler)

	if *https {
		err = http.ListenAndServeTLS(":"+*port, *cert, *key, nil)
	} else {
		err = http.ListenAndServe(":"+*port, nil)
	}
	if err != nil {
		fmt.Println("ListenAndServe: ", err)
	}
}

// ShowAccessLog prints access logs
func ShowAccessLog(req *http.Request, statusCode int) {
	const layout = "[ 2006-01-02 15:04:05 ]"
	fmt.Printf("%s %-4s %d %s\n", time.Now().Format(layout), req.Method, statusCode, req.URL.Path)
}

// ProcessUploadFile stores the uploaded file
func ProcessUploadFile(req *http.Request) error {
	err := req.ParseMultipartForm(1 << 30)
	if err != nil {
		return err
	}
	uploadFile, header, err := req.FormFile("uploadfile")
	if err != nil {
		if err == http.ErrMissingFile {
			return nil
		}
		return err
	}

	defer uploadFile.Close()
	filename := Directory + req.URL.Path + "/" + header.Filename
	saveFile, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer saveFile.Close()
	_, err = io.Copy(saveFile, uploadFile)
	if err != nil {
		return err
	}

	return nil
}

// ShowFileList shows the HTML file list
func ShowFileList(w http.ResponseWriter, req *http.Request, dir string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	// Make sure the tail is '/'
	slashCheck := ""
	if !strings.HasSuffix(req.URL.Path, "/") {
		slashCheck = "/"
	}

	// Show file list
	responseString := "<html><body> <h3> Directory Listing for " + req.URL.Path[1:] + "/ </h3> <br/> <hr> <ul>"
	for _, f := range files {
		if f.Name()[0] != '.' {
			if f.IsDir() {
				responseString += "<li><a href=\"http://" + req.Host + req.URL.Path + slashCheck + f.Name() + "\">" + f.Name() + "/" + "</a></li>"
			} else {
				responseString += "<li><a href=\"http://" + req.Host + req.URL.Path + slashCheck + f.Name() + "\">" + f.Name() + "</a></li>"
			}
		}
	}

	p := req.URL.Path

	// Display link to parent directory
	if len(p) > 1 {
		base := path.Base(p)
		slice := len(p) - len(base) - 1
		url := "/"

		if slice > 1 {
			url = req.URL.Path[:slice]
			url = strings.TrimRight(url, "/") // Remove extra / at the end
		}

		responseString += "<br/><a href=\"http://" + req.Host + url + "\">Parent directory</a>"
	}

	responseString += "</ul><br/><hr/>"
	responseString += `<form enctype="multipart/form-data" action="http://`
	responseString += req.Host + req.URL.Path
	responseString += `" method="post">
							<table><tr>
							<td><input type="file" name="uploadfile"/></td>
							<td><input type="submit" value="upload" /></td><br>
							</tr><tr>
							</tr><tr>
							<td><select name=filelist>
							<option value=""></option>`
	for _, f := range files {
		if f.Name()[0] != '.' {
			if f.IsDir() {
				responseString += "<option value=\"" + f.Name() + "\">" + f.Name() + "/" + "</option>"
			} else {
				responseString += "<option value=\"" + f.Name() + "\">" + f.Name() + "</option>"
			}
		}
	}
	responseString += `	</select></td>
							<td><input type="submit" value="remove" /></td>
							<tr></table></form></body></html>`

	if _, err = w.Write([]byte(responseString)); err != nil {
		return err
	}

	return nil

}

// Handler processes HTTP requests
func Handler(w http.ResponseWriter, req *http.Request) {

	statusCode := http.StatusOK
	defer ShowAccessLog(req, statusCode)

	filename := Directory + req.URL.Path
	file, err := os.Stat(filename)

	// 404 if file doesn't exist
	if err != nil {
		_, err = w.Write([]byte("404 Not Found"))
		statusCode = http.StatusNotFound
		//ShowAccessLog(req, http.StatusNotFound)
		return
	}

	// Serve directory
	if file.IsDir() {

		// For POST operation
		if req.Method == "POST" {

			// New Directory
			newDir := req.FormValue("newdir")
			if newDir != "" {
				os.Mkdir(Directory+req.URL.Path+"/"+newDir, 0666)
			}

			// Remove file/directory
			removeFile := req.FormValue("filelist")
			if removeFile != "" {
				os.RemoveAll(Directory + req.URL.Path + "/" + removeFile)
			}

			// Upload file
			err = ProcessUploadFile(req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				statusCode = http.StatusInternalServerError
			}
		}

		// Get file list
		err = ShowFileList(w, req, filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			statusCode = http.StatusInternalServerError
		}
		return
	}

	// File exists and is no directory; Serve the file
	http.ServeFile(w, req, filename)
}
