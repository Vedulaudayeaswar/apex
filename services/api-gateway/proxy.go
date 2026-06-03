package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/apex-retail/shared/pkg/config"
	"github.com/gin-gonic/gin"
)

func detectionURL() string {
	return config.Get("DETECTION_URL", "http://detection-service:8090")
}

func (gw *Gateway) uploadVideo(c *gin.Context) {
	file, header, err := c.Request.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing video file"})
		return
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("video", header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}
	if _, err := io.Copy(part, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}
	writer.Close()

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		detectionURL()+"/upload", body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "detection service unavailable"})
		return
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, "application/json", data)
}

func (gw *Gateway) proxyDetection(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		url := detectionURL() + path
		resp, err := http.Get(url)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "detection unavailable"})
			return
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		c.Data(resp.StatusCode, "application/json", data)
	}
}
