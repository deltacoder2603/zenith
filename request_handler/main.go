package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type DeployResponse struct {
	Repo      string `json:"repo"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Bucket    string `json:"bucket"`
	File      string `json:"file"`
	Timestamp string `json:"timestamp"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"}, // change this to your frontend origin
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/deploy", HandleDeployRequest)
	r.POST("/deploy", HandleDeployRequest)

	// Create required directories
	os.MkdirAll("./deployed", os.ModePerm)

	r.Run(":8080")
}

// Download file from MinIO/B2 storage
func downloadFile(filepath string, url string) error {
	// Create a new MinIO client
	endpoint := os.Getenv("B2_ENDPOINT")
	if endpoint == "" {
		endpoint = "your_endpoint"
	}

	accessKey := os.Getenv("B2_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "your_access_key"
	}

	secretKey := os.Getenv("B2_SECRET_KEY")
	if secretKey == "" {
		secretKey = "your_secret_key"
	}

	bucketName := os.Getenv("B2_BUCKET")
	if bucketName == "" {
		bucketName = "your_bucket_name"
	}

	// Extract object name from URL
	parts := strings.Split(url, "/")
	objectName := parts[len(parts)-1]

	log.Printf("Downloading from bucket: %s, object: %s", bucketName, objectName)

	// Initialize MinIO client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Download the object
	err = minioClient.FGetObject(context.Background(), bucketName, objectName, filepath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to download object: %w", err)
	}

	// Check if the downloaded file is valid
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		return fmt.Errorf("failed to stat downloaded file: %w", err)
	}

	log.Printf("Downloaded file size: %d bytes", fileInfo.Size())
	return nil
}

// HandleDeployRequest processes deployment requests
func HandleDeployRequest(c *gin.Context) {
	var urlFromQuery string

	// Handle both GET and POST requests
	if c.Request.Method == "GET" {
		urlFromQuery = c.Query("url")
	} else {
		var requestBody struct {
			URL string `json:"url"`
		}
		if err := c.BindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON body"})
			return
		}
		urlFromQuery = requestBody.URL
	}

	if urlFromQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'url' parameter"})
		return
	}

	// Step 1: Send to /upload
	log.Printf("Sending request to upload service: %s", urlFromQuery)
	uploadResp, err := sendPost("http://localhost:8081/upload", map[string]string{"url": urlFromQuery})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Log the upload response for debugging
	log.Printf("Upload response: %s", string(uploadResp))

	var deployData DeployResponse
	if err := json.Unmarshal(uploadResp, &deployData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Invalid upload response: %v", err)})
		return
	}

	// Extract repo name from URL if missing in response
	if deployData.Repo == "" {
		// Try to extract repo name from URL
		parts := strings.Split(urlFromQuery, "/")
		if len(parts) > 1 {
			repoName := parts[len(parts)-1]
			// Remove .git suffix if present
			repoName = strings.TrimSuffix(repoName, ".git")
			log.Printf("Repo name missing in response, extracted from URL: %s", repoName)
			deployData.Repo = repoName
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Upload response missing repo name and couldn't extract from URL"})
			return
		}
	}

	// Step 2: Send to /build
	buildPayload := map[string]interface{}{
		"repo":         deployData.Repo,
		"use_template": true,
		"template":     "create-react-app",
	}
	buildResp, err := sendPost("http://localhost:8082/build", buildPayload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Step 3: Download ZIP from Backblaze
	bucket := os.Getenv("B2_BUCKET")
	if bucket == "" {
		bucket = "zenith123" // Default bucket name
	}

	fileName := deployData.Repo + "-build.zip" // Default file name
	if deployData.File != "" {
		fileName = deployData.File
	}

	// We'll use the object name directly instead of constructing a URL
	zipFile := "./build.zip"
	if err := downloadFile(zipFile, fileName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Download failed: %v", err)})
		return
	}

	// Check if the downloaded file is valid
	fileInfo, err := os.Stat(zipFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to stat downloaded file: %v", err)})
		return
	}
	if fileInfo.Size() == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Downloaded file is empty"})
		return
	}
	log.Printf("Downloaded file size: %d bytes", fileInfo.Size())

	// Step 4: Unzip
	unzipPath := fmt.Sprintf("./deployed/%s", deployData.Repo)
	if err := unzip(zipFile, unzipPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Unzip failed: %v", err)})
		return
	}

	// Step 4.5: Build the project if it's not already built
	buildDir := unzipPath
	if _, err := os.Stat(filepath.Join(unzipPath, "package.json")); err == nil {
		log.Printf("Found package.json, building the project...")

		// Install dependencies
		installCmd := exec.Command("npm", "install")
		installCmd.Dir = unzipPath
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			log.Printf("Warning: npm install failed: %v", err)
		} else {
			// Build the project
			buildCmd := exec.Command("npm", "run", "build")
			buildCmd.Dir = unzipPath
			buildCmd.Stdout = os.Stdout
			buildCmd.Stderr = os.Stderr
			if err := buildCmd.Run(); err != nil {
				log.Printf("Warning: npm build failed: %v", err)
			} else {
				// Check for common build output directories
				for _, dir := range []string{"dist", "build", "out"} {
					if _, err := os.Stat(filepath.Join(unzipPath, dir)); err == nil {
						buildDir = filepath.Join(unzipPath, dir)
						log.Printf("Using build directory: %s", buildDir)
						break
					}
				}
			}
		}
	}

	// Step 5: Serve static site on a separate port
	go func() {
		serveStaticSite(buildDir)
	}()

	// Step 6: Start ngrok and get public URL
	publicURL, err := startNgrok("8090") // Changed to use the static site port
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ngrok failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "App deployed successfully",
		"repo":        deployData.Repo,
		"public_url":  publicURL,
		"buildResult": json.RawMessage(buildResp),
	})
}

// Helper function to send POST requests
func sendPost(url string, payload interface{}) ([]byte, error) {
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("POST to %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Function to unzip files
func unzip(src string, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	os.MkdirAll(dest, 0755)

	log.Printf("Unzipping %d files to %s", len(reader.File), dest)

	for _, f := range reader.File {
		path := filepath.Join(dest, f.Name)
		log.Printf("Extracting: %s", f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		dstFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}

		_, err = io.Copy(dstFile, srcFile)

		dstFile.Close()
		srcFile.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// Updated function to serve static site on a specific port
func serveStaticSite(folder string) {
	// Create a new HTTP mux
	mux := http.NewServeMux()

	// Create the directory if it doesn't exist
	os.MkdirAll(folder, 0755)

	// Look for index.html in the folder or any subdirectories
	var indexPath string
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Base(path) == "index.html" {
			indexPath = path
			return filepath.SkipDir // Stop walking once we find index.html
		}
		return nil
	})

	if err != nil {
		log.Printf("Error walking directory: %v", err)
	}

	if indexPath != "" {
		// If we found index.html, use its directory as the build directory
		buildDir := filepath.Dir(indexPath)
		log.Printf("Found index.html at: %s, serving from: %s", indexPath, buildDir)

		// Create a file server handler
		fs := http.FileServer(http.Dir(buildDir))

		// Create a custom handler to serve index.html for all routes
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Check if the requested path exists
			path := filepath.Join(buildDir, r.URL.Path)
			_, err := os.Stat(path)

			// If the path doesn't exist or is a directory, serve index.html
			if os.IsNotExist(err) || r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, "/") {
				log.Printf("Serving index.html for path: %s", r.URL.Path)
				http.ServeFile(w, r, indexPath)
				return
			}

			// Otherwise, serve the requested file
			log.Printf("Serving file: %s", r.URL.Path)
			fs.ServeHTTP(w, r)
		})
	} else {
		// Fallback to serving the folder directly
		log.Printf("No index.html found, serving entire folder: %s", folder)
		fs := http.FileServer(http.Dir(folder))
		mux.Handle("/", fs)
	}

	// Start the static file server on port 8181
	log.Printf("Starting static file server on port 8181")
	if err := http.ListenAndServe(":8181", mux); err != nil {
		log.Printf("Static server error: %v", err)
	}
}

// Updated startNgrok function with improved error handling and robustness
func startNgrok(port string) (string, error) {
	// Check if ngrok is already running
	_, err := http.Get("http://localhost:4040/api/tunnels")
	if err == nil {
		// Ngrok is already running, get the URL
		return getNgrokURL()
	}

	// Get authtoken from environment variable or use the one in the config
	authToken := os.Getenv("NGROK_AUTHTOKEN")
	if authToken == "" {
		authToken = "your_ngrok_authtoken"
	}

	// Start ngrok with proper parameters
	cmd := exec.Command("ngrok", "http", port, "--authtoken", authToken)

	// Create pipes for command output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start ngrok: %v", err)
	}

	// Process command output in background
	go func() {
		scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
		for scanner.Scan() {
			log.Printf("ngrok: %s", scanner.Text())
		}
	}()

	// Wait for ngrok to initialize (check for API availability)
	log.Println("Waiting for ngrok to start...")
	startTime := time.Now()
	timeout := 30 * time.Second

	for {
		if time.Since(startTime) > timeout {
			cmd.Process.Kill()
			return "", fmt.Errorf("timed out waiting for ngrok to start")
		}

		time.Sleep(500 * time.Millisecond)

		// Check if ngrok API is available
		_, err := http.Get("http://localhost:4040/api/tunnels")
		if err == nil {
			break
		}
	}

	// Get the public URL
	return getNgrokURL()
}

// Helper function to get ngrok URL from API
func getNgrokURL() (string, error) {
	resp, err := http.Get("http://localhost:4040/api/tunnels")
	if err != nil {
		return "", fmt.Errorf("failed to get ngrok tunnels: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read ngrok API response: %v", err)
	}

	var data struct {
		Tunnels []struct {
			Name      string `json:"name"`
			PublicURL string `json:"public_url"`
			Proto     string `json:"proto"`
		} `json:"tunnels"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse ngrok API response: %v", err)
	}

	if len(data.Tunnels) == 0 {
		return "", fmt.Errorf("no ngrok tunnels found")
	}

	// Find https tunnel if available
	for _, tunnel := range data.Tunnels {
		if strings.HasPrefix(tunnel.PublicURL, "https://") {
			return tunnel.PublicURL, nil
		}
	}

	// Otherwise return the first tunnel
	return data.Tunnels[0].PublicURL, nil
}
