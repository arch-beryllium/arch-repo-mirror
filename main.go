package main

import (
	"fmt"
	"io"
	"io/ioutil"
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
		Repos:       []string{"danctnix", "phosh", "pine64"},
	},
	{
		BaseAddress: "https://repo.lohl1kohl.de/$repo/$arch",
		Format:      "tar.xz",
		Repos:       []string{"beryllium"},
	},
	{
		BaseAddress: "https://raw.githubusercontent.com/arch-beryllium/plasma-mobile-packages/packages",
		Format:      "tar.xz",
		Repos:       []string{"plasma-mobile"},
	},
	{
		BaseAddress: "http://mirror.archlinuxarm.org/$arch/$repo",
		Format:      "tar.gz",
		Repos:       []string{"alarm", "aur", "community", "core", "extra"},
	},
}

type Config struct {
	BaseAddress string
	Format      string
	Repos       []string
}

func main() {
	for _, config := range configs {
		for _, repo := range config.Repos {
			dirPath := filepath.Join("mirror", repo, "aarch64")
			if _, err := os.Stat(dirPath); os.IsNotExist(err) {
				err = os.MkdirAll(dirPath, 0755)
				if err != nil {
					fmt.Printf("Failed to create %s: %v\n", dirPath, err)
					os.Exit(1)
				}
			}
			dbFilePath := filepath.Join(dirPath, fmt.Sprintf("%s.%s", repo, config.Format))
			dbFileURL := fmt.Sprintf("%s/%s.db", buildURL(config.BaseAddress, repo), repo)
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
				fmt.Printf("%v\n", err)
				os.Exit(1)
			}

			err = archiver.Unarchive(dbFilePath, tmpDir)
			if err != nil {
				fmt.Printf("Failed to read %s: %v\n", dbFilePath, err)
				os.Exit(1)
			}
			var dirs []os.FileInfo
			dirs, err = ioutil.ReadDir(tmpDir)
			if err != nil {
				fmt.Printf("Failed to read dir %s: %v\n", tmpDir, err)
				os.Exit(1)
			}
			var fileNames []string
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
				fileNames = append(fileNames, fileName)
				filePath := filepath.Join(dirPath, fileName)
				fileURL := fmt.Sprintf("%s/%s", buildURL(config.BaseAddress, repo), fileName)
				if _, err = os.Stat(filePath); os.IsNotExist(err) {
					err = downloadFile(filePath, fileURL)
					if err != nil {
						fmt.Printf("Failed to download %s: %v\n", fileURL, err)
						os.Exit(1)
					}
				}
			}
			files, err := ioutil.ReadDir(dirPath)
			if err != nil {
				fmt.Printf("Failed to read dir %s: %v\n", tmpDir, err)
				os.Exit(1)
			}
			for _, file := range files {
				if file.Name() == fmt.Sprintf("%s.db", repo) || file.Name() == fmt.Sprintf("%s.%s", repo, config.Format) {
					continue
				}
				found := false
				for _, fileName := range fileNames {
					if file.Name() == fileName {
						found = true
					}
				}
				if !found {
					fmt.Printf("Deleting %s\n", file.Name())
					err = os.Remove(filepath.Join(dirPath, file.Name()))
					if err != nil {
						fmt.Printf("Failed to remove %s: %v\n", tmpDir, err)
						os.Exit(1)
					}
				}
			}
			err = os.RemoveAll(tmpDir)
			if err != nil {
				fmt.Printf("Failed to remove %s: %v\n", tmpDir, err)
				os.Exit(1)
			}
		}
	}
}

func printDownloadPercent(done chan chan struct{}, path string, expectedSize int64) {
	var completedCh chan struct{}
	for {
		fi, err := os.Stat(path)
		if err != nil {
			fmt.Printf("%v\n", err)
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
	fmt.Printf("\033[2K\rDownload completed in %.2fs\n", elapsed.Seconds())
	return nil
}

func buildURL(baseAddress, repo string) string {
	baseAddress = strings.ReplaceAll(baseAddress, "$repo", repo)
	baseAddress = strings.ReplaceAll(baseAddress, "$arch", "aarch64")
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
