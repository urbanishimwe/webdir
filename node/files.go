package node

import (
	"log"
	"os"
	"path/filepath"
)

func initBaseDir(node *NodeConfig) error {
	if node.BaseFilePath == "" {
		node.BaseFilePath = filepath.Join(os.Getenv("HOME"), "webdir")
	}

	_, err := os.Stat(node.BaseFilePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(node.BaseFilePath, 0777)
}

func addOwnedFiles(node *NodeConfig) error {
	return filepath.WalkDir(node.BaseFilePath, func(path string, dirEntry os.DirEntry, err error) error {
		if err != nil {
			log.Printf("readBaseDir walking %q error: %q\n", path, err)
			return nil
		}

		// Since we only share files, we will skip all other sub-directories in our base directory
		if dirEntry.IsDir() {
			return filepath.SkipDir
		}

		fileCont, _ := readFile(node, dirEntry.Name())
		node.ClientUpdateFile(UpdateFileContent{Name: dirEntry.Name(), Content: string(fileCont)})
		return nil
	})
}

func readFile(nd *NodeConfig, fileName string) ([]byte, error) {
	return os.ReadFile(filepath.Join(nd.BaseFilePath, fileName))
}

func writeFile(nd *NodeConfig, fileName string, data []byte) error {
	return os.WriteFile(filepath.Join(nd.BaseFilePath, fileName), data, 0666)
}

func deleteFile(nd *NodeConfig, fileName string) error {
	return os.Remove(filepath.Join(nd.BaseFilePath, fileName))
}

func createFile(nd *NodeConfig, fileName string) error {
	f, err := os.OpenFile(filepath.Join(nd.BaseFilePath, fileName), os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	return f.Close()
}
