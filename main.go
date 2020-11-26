package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
)

var configs = []Config{
	{
		BaseAddress: "https://p64.arikawa-hi.me/$repo/$arch",
		Format:      "tar.xz",
		Repos: map[string][]string{
			"danctnix": {"aarch64"},
			"phosh":    {"aarch64"},
			"pine64":   {"aarch64"},
		},
	},
	{
		BaseAddress: "https://repo.lohl1kohl.de/$repo/$arch",
		Format:      "tar.xz",
		Repos: map[string][]string{
			"beryllium": {"aarch64"},
		},
	},
	{
		BaseAddress: "https://ftp.halifax.rwth-aachen.de/manjaro/arm-unstable/$repo/$arch",
		Format:      "tar.gz",
		Repos: map[string][]string{
			"mobile": {"aarch64"},
		},
	},
	{
		BaseAddress: "https://ftp.halifax.rwth-aachen.de/archlinux-arm/$arch/$repo",
		Format:      "tar.gz",
		Repos: map[string][]string{
			"alarm":     {"aarch64"},
			"aur":       {"aarch64"},
			"community": {"aarch64"},
			"core":      {"aarch64"},
			"extra":     {"aarch64"},
		},
	},
}

type Config struct {
	BaseAddress string              `yaml:"base_address"`
	Format      string              `yaml:"format"`
	Repos       map[string][]string `yaml:"repos"`
}

func main() {
	for _, config := range configs {
		for repo, architectures := range config.Repos {
			for _, architecture := range architectures {
				dirPath := filepath.Join("mirror", repo, architecture)
				if _, err := os.Stat(dirPath); os.IsNotExist(err) {
					err = os.MkdirAll(dirPath, 0755)
					if err != nil {
						fmt.Printf("Failed to create %s: %v\n", dirPath, err)
						os.Exit(1)
					}
				}
				dbFilePath := filepath.Join(dirPath, fmt.Sprintf("%s.%s", repo, config.Format))
				dbFileURL := fmt.Sprintf("%s/%s.db", buildURL(config.BaseAddress, repo, architecture), repo)
				err := downloadFile(dbFilePath, dbFileURL)
				if err != nil {
					fmt.Printf("Failed to download %s: %v\n", dbFileURL, err)
					os.Exit(1)
				}
				err = copyFile(dbFilePath, filepath.Join(dirPath, fmt.Sprintf("%s.db", repo)))
				if err != nil {
					fmt.Printf("Failed to copy %s : %v\n", dbFilePath, err)
					os.Exit(1)
				}
				var tmpDir string
				tmpDir, err = ioutil.TempDir("", "arch-repo-mirror-*")
				if err != nil {
					log.Fatal(err)
				}

				err = archiver.Unarchive(dbFilePath, tmpDir)
				if err != nil {
					fmt.Printf("Failed to read %s: %v", dbFilePath, err)
					os.Exit(1)
				}
				var dirs []os.FileInfo
				dirs, err = ioutil.ReadDir(tmpDir)
				for _, dir := range dirs {
					descFilePath := filepath.Join(tmpDir, dir.Name(), "desc")
					var content []byte
					content, err = ioutil.ReadFile(descFilePath)
					if err != nil {
						fmt.Printf("Failed to read %s: %v\n", descFilePath, err)
						os.Exit(1)
					}
					fileName := ""
					lines := strings.Split(string(content), "\n")
					for i, line := range lines {
						if line == "%FILENAME%" {
							fileName = lines[i+1]
							break
						}
					}
					filePath := filepath.Join(dirPath, fileName)
					fileURL := fmt.Sprintf("%s/%s", buildURL(config.BaseAddress, repo, architecture), fileName)
					if _, err = os.Stat(filePath); os.IsNotExist(err) {
						err = downloadFile(filePath, fileURL)
						if err != nil {
							fmt.Printf("Failed to download %s: %v\n", fileURL, err)
							os.Exit(1)
						}
					}
				}
				err = os.RemoveAll(tmpDir)
				if err != nil {
					fmt.Printf("Failed to remove %s: %v", tmpDir, err)
					os.Exit(1)
				}
			}
		}
	}
}

func printDownloadPercent(done chan chan struct{}, path string, expectedSize int64) {
	var completedCh chan struct{}
	for {
		fi, err := os.Stat(path)
		if err != nil {
			fmt.Printf("%v", err)
		}

		size := fi.Size()

		if size == 0 {
			size = 1
		}

		var percent = float64(size) / float64(expectedSize) * 100

		fmt.Printf("\033[2K\r %.0f %% / 100 %%", percent)

		if completedCh != nil {
			close(completedCh)
			return
		}

		select {
		case completedCh = <-done:
		case <-time.After(time.Second / 60):
		}
	}
}

func downloadFile(filepath string, url string) error {
	fmt.Println(url)

	start := time.Now()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	expectedSize, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return errors.Wrap(err, "failed to get Content-Length header")
	}

	doneCh := make(chan chan struct{})
	go printDownloadPercent(doneCh, filepath, int64(expectedSize))

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	doneCompletedCh := make(chan struct{})
	doneCh <- doneCompletedCh
	<-doneCompletedCh

	elapsed := time.Since(start)
	log.Printf("\033[2K\rDownload completed in %.2fs", elapsed.Seconds())
	return nil
}

func buildURL(baseAddress, repo, architecture string) string {
	baseAddress = strings.ReplaceAll(baseAddress, "$repo", repo)
	baseAddress = strings.ReplaceAll(baseAddress, "$arch", architecture)
	return baseAddress
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	file, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, in)
	return err
}
