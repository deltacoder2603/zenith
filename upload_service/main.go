package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type DeployRequest struct {
	URL string `json:"url" binding:"required,url"`
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}

	requiredEnvVars := []string{"GITHUB_TOKEN", "B2_ACCESS_KEY", "B2_SECRET_KEY"}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("Error: %s environment variable is not set", envVar)
		}
	}

	if err := os.MkdirAll("./tmp", 0755); err != nil {
		log.Fatalf("Error creating tmp directory: %v", err)
	}

	router := gin.Default()

	router.POST("/upload", handleDeploy)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleDeploy(c *gin.Context) {
	var req DeployRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if !strings.HasPrefix(req.URL, "https://github.com/") {
		c.JSON(400, gin.H{"error": "URL must be a valid GitHub repository URL"})
		return
	}

	log.Printf("Deploying repository: %s", req.URL)

	repoPath, repoName, err := CloneRepoWithToken(req.URL)
	if err != nil {
		c.JSON(500, gin.H{"error": "Clone failed: " + err.Error()})
		return
	}
	log.Printf("Repository cloned to: %s", repoPath)

	defer func() {
		if err := os.RemoveAll(repoPath); err != nil {
			log.Printf("Warning: Failed to clean up repo directory: %v", err)
		}
	}()

	zipPath := filepath.Join("./tmp", repoName+".zip")
	if err := ZipFolder(repoPath, zipPath); err != nil {
		c.JSON(500, gin.H{"error": "Zipping failed: " + err.Error()})
		return
	}
	log.Printf("Repository zipped to: %s", zipPath)

	defer func() {
		if err := os.Remove(zipPath); err != nil {
			log.Printf("Warning: Failed to clean up zip file: %v", err)
		}
	}()

	bucketName := "your_bucket_name"
	objectName := repoName + ".zip"

	if err := UploadFileToB2(zipPath, bucketName, objectName); err != nil {
		c.JSON(500, gin.H{"error": "Upload failed: " + err.Error()})
		return
	}

	log.Printf("Successfully uploaded %s to B2 bucket %s", objectName, bucketName)
	c.JSON(200, gin.H{
		"message":   "Repo uploaded to B2 successfully!",
		"bucket":    bucketName,
		"file":      objectName,
		"repo":      repoName,
		"timestamp": time.Now().Format(time.RFC3339),
	})

}

func CloneRepoWithToken(repoURL string) (string, string, error) {
	tempDir := "./tmp"
	token := os.Getenv("GITHUB_TOKEN")

	trimmed := strings.TrimPrefix(repoURL, "https://github.com/")
	authURL := fmt.Sprintf("https://%s@github.com/%s", token, trimmed)

	repoName := strings.TrimSuffix(filepath.Base(trimmed), ".git")
	if !strings.HasSuffix(trimmed, ".git") {
		authURL += ".git"
	}

	repoFolder := filepath.Join(tempDir, repoName)

	if _, err := os.Stat(repoFolder); err == nil {
		if err := os.RemoveAll(repoFolder); err != nil {
			return "", "", fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	cmd := exec.Command("git", "clone", "--depth", "1", authURL, repoFolder)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("git clone failed: %w - output: %s", err, output)
	}

	return repoFolder, repoName, nil
}

func ZipFolder(source, target string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && (info.Name() == ".git" || strings.Contains(path, "/.git/")) {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		relPath = filepath.ToSlash(relPath)

		writer, err := archive.Create(relPath)
		if err != nil {
			return fmt.Errorf("failed to create entry in zip: %w", err)
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

func UploadFileToB2(filePath, bucket, objectName string) error {
	endpoint := "your_endpoint"
	accessKey := os.Getenv("B2_ACCESS_KEY")
	secretKey := os.Getenv("B2_SECRET_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
		Region: "your_endpoint_region",
	})
	if err != nil {
		return fmt.Errorf("failed to create B2 client: %w", err)
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket '%s' does not exist", bucket)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	log.Printf("Uploading %s (%d bytes) to B2...", objectName, fileInfo.Size())
	_, err = client.FPutObject(
		ctx,
		bucket,
		objectName,
		filePath,
		minio.PutObjectOptions{ContentType: "application/zip"},
	)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	return nil
}
