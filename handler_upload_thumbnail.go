package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to parse video id", err)
		return
	}
	
	randomBytes := make([]byte, 32)
    _, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to generate random id", err)
		return
	}

	randomFileName := base64.RawURLEncoding.EncodeToString(randomBytes)
	
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header ,err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to get video meta data", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "you don't own this video", nil)
		return
	}

	parts := strings.Split(mediaType, "/")
	if len(parts) < 2 {
		respondWithError(w, http.StatusBadRequest, "invalid media type", nil)
		return
	}

	imgFilePath := filepath.Join(cfg.assetsRoot, randomFileName + "." + parts[1])
	thumbnailURL := fmt.Sprintf(
		"http://localhost:%s/assets/%s.%s",
		cfg.port, randomFileName, parts[1],
	)
	video.ThumbnailURL = &thumbnailURL

	imgFile, err := os.Create(imgFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to create file", err)
		return
	}
	defer imgFile.Close()

	_, err = io.Copy(imgFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to copy file", err)
		return
	}


	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
