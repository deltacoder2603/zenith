package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var buildMutex sync.Mutex
var ErrRepoNotFound = errors.New("repository not found")

type BuildRequest struct {
	RepoName    string `json:"repo" binding:"required"`
	UseTemplate bool   `json:"use_template"`
	Template    string `json:"template"`
}

func main() {
	godotenv.Load() // Ignore error, use env vars if available
	os.MkdirAll("tmp", os.ModePerm)

	router := gin.Default()
	router.POST("/build", handleBuildRequest)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	router.Run(":" + port)
}

func handleBuildRequest(c *gin.Context) {
	var req BuildRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error(), "status": "error"})
		return
	}

	if strings.Contains(req.RepoName, "/") || strings.Contains(req.RepoName, "..") {
		c.JSON(400, gin.H{"error": "Invalid repository name", "status": "error"})
		return
	}

	if req.Template == "" {
		req.Template = "create-react-app"
	}

	buildMutex.Lock()
	defer buildMutex.Unlock()

	var createdNew bool
	var err error

	err = HandleBuild(req.RepoName)

	if errors.Is(err, ErrRepoNotFound) && (req.UseTemplate || shouldCreateFromTemplate()) {
		fmt.Printf("Repository %s not found, creating from template %s\n", req.RepoName, req.Template)
		err = CreateFromTemplate(req.RepoName, req.Template)
		if err == nil {
			createdNew = true
		}
	}

	if err != nil {
		if errors.Is(err, ErrRepoNotFound) {
			c.JSON(404, gin.H{
				"error":   err.Error(),
				"status":  "not_found",
				"message": "Repository not found. Add 'use_template':true to create from template.",
			})
		} else {
			c.JSON(500, gin.H{"error": err.Error(), "status": "error"})
		}
		return
	}

	message := "Build completed and uploaded successfully"
	if createdNew {
		message = "Created new project from template and built successfully"
	}

	c.JSON(200, gin.H{
		"message":      message,
		"status":       "success",
		"created_from": ternary(createdNew, req.Template, ""),
	})
}

func ternary(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

func shouldCreateFromTemplate() bool {
	autoCreate := os.Getenv("AUTO_CREATE_FROM_TEMPLATE")
	return strings.ToLower(autoCreate) == "true" || autoCreate == "1"
}

func HandleBuild(repoName string) error {
	bucket := getEnvOrDefault("B2_BUCKET", "zenith123")
	zipFile := repoName + ".zip"
	downloadPath := filepath.Join("tmp", zipFile)
	unzipPath := filepath.Join("tmp", repoName)
	buildOutput := filepath.Join(unzipPath, "build")
	buildZipPath := filepath.Join("tmp", repoName+"-build.zip")

	client := getMinioClient()
	_, err := client.StatObject(context.Background(), bucket, zipFile, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return fmt.Errorf("%w: %s not found in bucket %s", ErrRepoNotFound, zipFile, bucket)
		}
		return fmt.Errorf("failed to check if file exists: %w", err)
	}

	if err := DownloadFromB2(bucket, zipFile, downloadPath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	if err := Unzip(downloadPath, unzipPath); err != nil {
		return fmt.Errorf("unzip failed: %w", err)
	}

	return buildProject(repoName, unzipPath, buildOutput, buildZipPath, bucket)
}

func CreateFromTemplate(repoName, templateName string) error {
	unzipPath := filepath.Join("tmp", repoName)
	buildOutput := filepath.Join(unzipPath, "build")
	buildZipPath := filepath.Join("tmp", repoName+"-build.zip")
	bucket := getEnvOrDefault("B2_BUCKET", "zenith123")

	var cmd *exec.Cmd
	switch templateName {
	case "create-react-app":
		cmd = exec.Command("npx", "create-react-app", unzipPath)
	case "next":
		cmd = exec.Command("npx", "create-next-app@latest", unzipPath, "--use-npm")
	case "vite":
		os.MkdirAll(unzipPath, os.ModePerm)
		cmd = exec.Command("npm", "init", "vite@latest", ".", "--", "--template", "react")
		cmd.Dir = unzipPath
	default:
		return fmt.Errorf("unsupported template: %s", templateName)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create project from template: %w", err)
	}

	templateZipPath := filepath.Join("tmp", repoName+".zip")
	if err := ZipFolder(unzipPath, templateZipPath); err != nil {
		return fmt.Errorf("failed to zip templated project: %w", err)
	}
	if err := UploadFileToB2(templateZipPath, bucket, repoName+".zip"); err != nil {
		return fmt.Errorf("failed to upload templated project: %w", err)
	}
	os.Remove(templateZipPath)

	return buildProject(repoName, unzipPath, buildOutput, buildZipPath, bucket)
}

func buildProject(repoName, unzipPath, buildOutput, buildZipPath, bucket string) error {
	if _, err := os.Stat(filepath.Join(unzipPath, "package.json")); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found in repository")
	}

	install := exec.Command("npm", "install")
	install.Dir = unzipPath
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("npm install failed: %w", err)
	}

	build := exec.Command("npm", "run", "build")
	build.Dir = unzipPath
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("npm build failed: %w", err)
	}

	if _, err := os.Stat(buildOutput); os.IsNotExist(err) {
		alternatives := []string{
			filepath.Join(unzipPath, "dist"),
			filepath.Join(unzipPath, "out"),
			filepath.Join(unzipPath, ".next"),
		}
		buildFound := false
		for _, alt := range alternatives {
			if _, err := os.Stat(alt); !os.IsNotExist(err) {
				buildOutput = alt
				buildFound = true
				break
			}
		}
		if !buildFound {
			return fmt.Errorf("build folder not found")
		}
	}

	if err := ZipFolder(buildOutput, buildZipPath); err != nil {
		return fmt.Errorf("zipping build folder failed: %w", err)
	}
	if err := UploadFileToB2(buildZipPath, bucket, repoName+"-build.zip"); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	os.RemoveAll(unzipPath)
	os.Remove(buildZipPath)
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func DownloadFromB2(bucket, objectName, destPath string) error {
	client := getMinioClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return client.FGetObject(ctx, bucket, objectName, destPath, minio.GetObjectOptions{})
}

func UploadFileToB2(filePath, bucket, objectName string) error {
	client := getMinioClient()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	_, err := client.FPutObject(ctx, bucket, objectName, filePath, minio.PutObjectOptions{
		ContentType: "application/zip",
	})
	return err
}

func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
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
		if err != nil || info.IsDir() {
			return err
		}
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		relPath = strings.ReplaceAll(relPath, string(filepath.Separator), "/")
		writer, err := archive.Create(relPath)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(writer, f)
		return err
	})
}

func getMinioClient() *minio.Client {
	endpoint := getEnvOrDefault("B2_ENDPOINT", "your_endpoint")
	accessKey := os.Getenv("B2_ACCESS_KEY")
	secretKey := os.Getenv("B2_SECRET_KEY")

	if accessKey == "" || secretKey == "" {
		panic("B2_ACCESS_KEY and B2_SECRET_KEY environment variables must be set")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStatic(accessKey, secretKey, "", credentials.SignatureV4),
		Secure: true,
	})
	if err != nil {
		panic(err)
	}
	return client
}
