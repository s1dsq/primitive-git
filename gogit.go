package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// what:
//  - create .git/ with enough to push to a git remote
//   - create master branch with one commit
//  - add and push remote with git command line

// how:
//  - create HEAD file "ref: refs/heads/master" at .git/HEAD
//  - create blob -> tree -> commit objects in .git/objects
//  - create ref file .git/refs/heads/master to point to commit hash

// references: Git Internals -> Object storage section in the progit book is
// a good starting point

var objectsDir = filepath.Join(".git", "objects")

func getObject(content, objectType string) (string, bytes.Buffer) {
	var header string
	var store string
	switch objectType {
	case "blob":
		header = "blob " + strconv.Itoa(len([]byte(content))) + "\000"
	case "tree":
		header = "tree " + strconv.Itoa(len([]byte(content))) + "\000"
	case "commit":
		header = "commit " + strconv.Itoa(len([]byte(content))) + "\000"
	default:
		fmt.Println("Unhandled object type", objectType)
	}
	store = header + content

	// the content of the object stored on disk
	var zlibContent bytes.Buffer
	w := zlib.NewWriter(&zlibContent)
	w.Write([]byte(store))
	w.Close()

	// used for creating object filename
	hash := sha1.Sum([]byte(store))
	return hex.EncodeToString(hash[:]), zlibContent
}

func createDir(dirName string) {
	if _, err := os.Stat(dirName); errors.Is(err, fs.ErrNotExist) {
		err := os.MkdirAll(dirName, 0755)
		if err != nil {
			log.Fatal("failed to create", dirName, ":", err)
		}
	}
}

func writeObject(dirName, fileName string, content []byte) error {
	createDir(dirName)
	filePath := filepath.Join(dirName, fileName)
	err := os.WriteFile(filePath, content, 0644)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	// a minimal git repo needs .git/HEAD (file) and .git/refs (directory)
	createDir(".git")
	err := os.WriteFile(filepath.Join(".git", "HEAD"), []byte("ref: refs/heads/master\n"), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// create a blob and write the object
	content := "what is up, doc?\n"
	blobHash, zlibContent := getObject(content, "blob")
	err = writeObject(filepath.Join(objectsDir, blobHash[0:2]), blobHash[2:], zlibContent.Bytes())
	if err != nil {
		log.Fatal("failed to write blob object", err)
	}

	// create a tree object pointing to the blob above
	// https://stackoverflow.com/questions/14790681/what-is-the-internal-format-of-a-git-tree-object
	//  tree [content size]\0[Entries having references to other trees and blobs]
	//  [mode] [file/folder name]\0[SHA-1 of referencing blob or tree]
	// 100644 - blob
	// 040000 - tree
	// if we have more than one entry for referencing blob/tree, it should be
	// sorted lexicographically based on path

	// concat hash of previous blob in binary format: https://stackoverflow.com/a/33039114
	decoded, _ := hex.DecodeString(blobHash)
	content = "100644 hello.txt\000" + string(decoded[:])
	treeHash, zlibContent := getObject(content, "tree")
	err = writeObject(filepath.Join(objectsDir, treeHash[0:2]), treeHash[2:], zlibContent.Bytes())
	if err != nil {
		log.Fatal("failed to write tree object", err)
	}

	// create commit object pointing to tree above
	// commit hashes are non-deterministic as they contain timestamp which changes
	// use fixed timestamp to make hash deterministic
	content = "tree " + treeHash + "\nauthor Siddharth Singh <me@s1dsq.com> 1708260537 +0530\ncommitter Siddharth Singh <me@s1dsq.com> 1708260537 +0530\n\nAdd hello.txt\n"
	commitHash, zlibContent := getObject(content, "commit")
	err = writeObject(filepath.Join(objectsDir, commitHash[0:2]), commitHash[2:], zlibContent.Bytes())
	if err != nil {
		log.Fatal("failed to write commit object", err)
	}

	// point "master" ref to the commit hash we just created
	createDir(".git/refs/heads")
	err = os.WriteFile(filepath.Join(".git", "refs", "heads", "master"), []byte(commitHash), 0644)
	if err != nil {
		log.Fatal("failed to write refs/heads", err)
	}
}
