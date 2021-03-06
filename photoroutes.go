package main

import (
	"bytes"
	"archive/zip"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/pixolous/pixolousAnalyze"
)

type galleryResponse struct {
	Images []string `json:"image"`
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	/* unzip received data */
	raw, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "malformed zip file", http.StatusBadRequest)
		return
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		http.Error(w, "malformed zip file", http.StatusBadRequest)
		return
	}

	/* get userid from context */
	userid := r.Context().Value("userid")
	if userid == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	/* get user's hash */
	userhash, err := GetUserHash(userid.(int))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	/* get list of files */
	for _, file := range reader.File {
		ext := filepath.Ext(file.Name)
		filehash := GenerateMD5(file.Name)

		/* write photo to drive */
        err = WriteImageFile(file, userhash, filehash+ext)
        if err != nil {
            http.Error(w, "erroring saving file", http.StatusInternalServerError); return
        }

		/* compute ahash */
		ahash := pixolousAnalyze.AHash(filepath.Join(ResourceDir, userhash, filehash+ext))

		/* write photo to db */
        err := NewPhoto(userid.(int), filehash+ext, ahash)
        if err != nil {
            http.Error(w, "error writing to db", http.StatusInternalServerError); return
        }
	}
}

func galleryHandler(w http.ResponseWriter, r *http.Request) {

	/* get userid from context */
	userid := r.Context().Value("userid")
	if userid == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	/* get user's hash */
	userhash, err := GetUserHash(userid.(int))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

    /* get all user's photos from db */
    photoData, err := GetUserPhotos(userid.(int))
    if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
    }

    /* group similar images */
    var groups [][]string = pixolousAnalyze.GetSimilarGrouped(photoData)

    /* filter groupped pics by using blur level */
    filtered := []string{}
    for _, group := range groups {
        bestBlur := -1000000
        bestPic := ""
        for _, picpath := range group {
            blurLevel := int(pixolousAnalyze.DetectBlur(filepath.Join(ResourceDir, userhash, picpath)))
            if blurLevel > bestBlur {
                bestBlur = blurLevel
                bestPic = picpath
            }
        }
        /* save least blurry pic */
        filtered = append(filtered, FILESURL+"/"+userhash+"/"+bestPic)
    }

	w.Header().Add("content-type", "application/json")
    json.NewEncoder(w).Encode(galleryResponse{Images: filtered})
}

func PhotoRoutes(mux *http.ServeMux) {
	mux.Handle("/photo/upload", RestrictMethod("POST", RestrictAuth(http.HandlerFunc(uploadHandler))))
	mux.Handle("/photo/gallery", RestrictMethod("GET", RestrictAuth(http.HandlerFunc(galleryHandler))))
}

