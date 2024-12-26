package main

import (
	"context"
	"fmt"
	"io"
	stdlog "log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/go-logr/stdr"
	"golang.org/x/oauth2/google"

	"github.com/theupdateframework/go-tuf/v2/metadata"
	"github.com/theupdateframework/go-tuf/v2/metadata/config"
	"github.com/theupdateframework/go-tuf/v2/metadata/updater"
)

// The following config is used to fetch a target from Jussi's GitHub repository example
const (
	metadataURL          = "https://sorayaormazabalmayo.github.io/TUF_Repository_YubiKey_Vault/metadata"
	targetsURL           = "https://sorayaormazabalmayo.github.io/TUF_Repository_YubiKey_Vault/targets"
	verbosity            = 4
	generateRandomFolder = false
	nameOfFile           = "index.json"
)

func main() {
	// set logger to stdout with info level
	metadata.SetLogger(stdr.New(stdlog.New(os.Stdout, "client_example", stdlog.LstdFlags)))
	stdr.SetVerbosity(verbosity)

	log := metadata.GetLogger()

	// initialize environment - temporary folders, etc.
	metadataDir, err := InitEnvironment()
	if err != nil {
		log.Error(err, "Failed to initialize environment")
	}

	// initialize client with Trust-On-First-Use
	err = InitTrustOnFirstUse(metadataDir)
	if err != nil {
		log.Error(err, "Trust-On-First-Use failed")
	}

	// The updater needs to be looking for new updates every x time
	for {
		// download the desired target
		targetFile, foundDesiredTargetLocally, err := DownloadTarget(metadataDir)

		if err != nil {
			log.Error(err, "Download target file failed")
		}

		if foundDesiredTargetLocally == 0 {

			err = os.WriteFile(filepath.Join(metadataDir, nameOfFile), targetFile, 0750)
			if err != nil {
				log.Error(err, "Error writing to file")
			}

		} else {
			fmt.Printf("\nThe local desired target is the most updated one \n")
		}

		// Service account key file
		serviceAccountKeyPath := "/home/sormazabal/artifact-downloader-key.json"

		// Construct Artifact Registry URL
		url := "https://artifactregistry.googleapis.com/download/v1/projects/polished-medium-445107-i9/locations/europe-southwest1/repositories/nebula-storage/files/nebula-package:2024.12.26-14.03.18:nebula-standalone:download?alt=media"

		fmt.Printf("Downloading binary from: %s\n", url)

		// Download the artifact without specifying the file type
		err = downloadArtifact(serviceAccountKeyPath, url)
		if err != nil {
			fmt.Printf("Failed to download binary: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Binary downloaded successfully!")
		time.Sleep(time.Second * 5)

	}
}

// InitEnvironment prepares the local environment - temporary folders, etc.
func InitEnvironment() (string, error) {
	var tmpDir string
	// get working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	if !generateRandomFolder {
		tmpDir = filepath.Join(cwd, "tmp")
		// create a temporary folder for storing the demo artifacts
		os.Mkdir(tmpDir, 0750)
	} else {
		// create a temporary folder for storing the demo artifacts
		tmpDir, err = os.MkdirTemp(cwd, "tmp")
		if err != nil {
			return "", fmt.Errorf("failed to create a temporary folder: %w", err)
		}
	}

	// create a destination folder for storing the downloaded target
	os.Mkdir(filepath.Join(tmpDir, "download"), 0750)
	return tmpDir, nil
}

// InitTrustOnFirstUse initialize local trusted metadata (Trust-On-First-Use)
func InitTrustOnFirstUse(metadataDir string) error {
	// check if there's already a local root.json available for bootstrapping trust
	_, err := os.Stat(filepath.Join(metadataDir, "root.json"))
	if err == nil {
		return nil
	}

	// download the initial root metadata so we can bootstrap Trust-On-First-Use
	rootURL, err := url.JoinPath(metadataURL, "1.root.json")
	if err != nil {
		return fmt.Errorf("failed to create URL path for 1.root.json: %w", err)
	}

	req, err := http.NewRequest("GET", rootURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	client := http.DefaultClient

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to executed the http request: %w", err)
	}

	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read the http request body: %w", err)
	}

	// write the downloaded root metadata to file
	err = os.WriteFile(filepath.Join(metadataDir, "root.json"), data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write root.json metadata: %w", err)
	}

	return nil
}

// DownloadTarget downloads the target file using Updater. The Updater refreshes the top-level metadata,
// get the target information, verifies if the target is already cached, and in case it
// is not cached, downloads the target file.
func DownloadTarget(localMetadataDir string) ([]byte, int, error) {
	// log := metadata.GetLogger()

	rootBytes, err := os.ReadFile(filepath.Join(localMetadataDir, "root.json"))
	if err != nil {
		return nil, 0, err
	}

	// create updater configuration
	cfg, err := config.New(metadataURL, rootBytes) // default config
	if err != nil {
		return nil, 0, err
	}
	cfg.LocalMetadataDir = localMetadataDir
	cfg.LocalTargetsDir = filepath.Join(localMetadataDir, "download")
	cfg.RemoteTargetsURL = targetsURL
	cfg.PrefixTargetsWithHash = true

	// create a new Updater instance
	up, err := updater.New(cfg)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create Updater instance: %w", err)
	}

	// try to build the top-level metadata
	err = up.Refresh()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to refresh trusted metadata: %w", err)
	}

	ti, err := up.GetTargetInfo(nameOfFile)
	if err != nil {
		return nil, 0, fmt.Errorf("getting info for target \"%s\": %w", nameOfFile, err)
	}

	path, tb, err := up.FindCachedTarget(ti, filepath.Join(localMetadataDir, nameOfFile))
	if err != nil {
		return nil, 0, fmt.Errorf("getting target cache: %w", err)
	}

	// fmt.Printf("\n%s\n", tb)
	// fmt.Printf("\n%s\n", path)

	if path != "" {
		// Cached version found
		fmt.Println("\U0001F34C CACHE HIT")
		return tb, 1, nil
	}

	fmt.Printf("\nThere is a new update:\n")
	userAnswer := gettingUserAnswer()

	if userAnswer == 1 {
		// Download of target is needed
		_, tb, err = up.DownloadTarget(ti, "", "")
		if err != nil {
			return nil, 0, fmt.Errorf("failed to download target file %s - %w", nameOfFile, err)
		}

		return tb, 0, nil
	}
	return tb, 1, nil
}

func gettingUserAnswer() int {

	var userAnswer int

	fmt.Printf("\n Do you want to download the new version?\n")

	fmt.Printf("\n Introduce your answer: \n")
	fmt.Println("------------------------------------------")
	fmt.Printf("\nFor YES => (1)")
	fmt.Printf("\nFor NO  => (2)\n")

	fmt.Scanf("%d", &userAnswer)

	return userAnswer

}

// downloadArtifact dynamically determines the file name and downloads the artifact
func downloadArtifact(keyFilePath, url string) error {
	// Authenticate using the service account key
	ctx := context.Background()
	creds, err := google.CredentialsFromJSON(ctx, readFile(keyFilePath), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return fmt.Errorf("failed to load service account credentials: %w", err)
	}

	// Create HTTP client with the token
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add Authorization header with Bearer token
	token, err := creds.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("failed to retrieve token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	// Perform the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download artifact, status code: %d", resp.StatusCode)
	}

	// Determine the file name from the Content-Disposition header or use a default name
	contentDisposition := resp.Header.Get("Content-Disposition")
	fileName := "downloaded-file"
	if contentDisposition != "" {
		_, params, err := mime.ParseMediaType(contentDisposition)
		if err == nil {
			if name, ok := params["filename"]; ok {
				fileName = name
			}
		}
	}

	fmt.Printf("Saving file as: %s\n", fileName)

	// Write the response to a file
	out, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// readFile reads the content of the service account key JSON file
func readFile(path string) []byte {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", path, err)
		os.Exit(1)
	}
	return content
}
