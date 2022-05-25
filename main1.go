package main

//Исходники задания для первого занятия у других групп https://github.com/t0pep0/GB_best_go

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type TargetFile struct {
	Path string
	Name string
}

type FileList map[string]TargetFile

type FileInfo interface {
	os.FileInfo
	Path() string
}

type fileInfo struct {
	os.FileInfo
	path string
}

func (fi fileInfo) Path() string {
	return fi.path
}

//Ограничить глубину поиска заданым числом, по SIGUSR2 увеличить глубину поиска на +2
func ListDirectory(ctx context.Context, dir string, sdepth int32, wg *sync.WaitGroup) ([]FileInfo, error) {
	var result []FileInfo
	if sdepth < 0 {

		return result, nil
	}
	select {
	case <-ctx.Done():
		return nil, nil
	default:
		wg.Add(1)
		sigUsr1 := make(chan os.Signal, 1)
		signal.Notify(sigUsr1, syscall.SIGUSR1)
		select {
		case <-sigUsr1:
			log.Printf("current directory: %s \nCurrent search depth %V", dir, sdepth)
		}
		time.Sleep(time.Second * 2)
		res, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range res {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				child, err := ListDirectory(ctx, path, sdepth-1, wg) //Дополнительно: вынести в горутину
				if err != nil {
					return nil, err
				}
				result = append(result, child...)
			} else {
				info, err := entry.Info()
				if err != nil {
					return nil, err
				}
				result = append(result, fileInfo{info, path})
			}
		}
		return result, nil
	}
}

func FindFiles(ctx context.Context, ext string, SearchDepth int32, wg *sync.WaitGroup) (FileList, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	files, err := ListDirectory(ctx, wd, SearchDepth, wg)
	if err != nil {
		return nil, err
	}
	fl := make(FileList, len(files))
	for _, file := range files {
		if filepath.Ext(file.Name()) == ext {
			fl[file.Name()] = TargetFile{
				Name: file.Name(),
				Path: file.Path(),
			}
		}
	}
	return fl, nil
}

func main() {
	var wg sync.WaitGroup
	var SearchDepth int32 = 2
	const wantExt = ".go"
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sigUsr2 := make(chan os.Signal, 1)
	signal.Notify(sigUsr2, syscall.SIGUSR2)

	waitCh := make(chan struct{})
	go func() {
		res, err := FindFiles(ctx, wantExt, SearchDepth, &wg)
		if err != nil {
			log.Printf("Error on search: %v\n", err)
			os.Exit(1)
		}
		for _, f := range res {
			fmt.Printf("\tName: %s\t\t Path: %s\n", f.Name, f.Path)
		}
		waitCh <- struct{}{}
	}()
	go func() {
		<-sigCh
		log.Println("Signal received, terminate...")
		cancel()
	}()
	<-waitCh
	for {
		select {
		case <-ctx.Done():
			log.Println("Done")
		case <-sigCh:
			log.Println("app terminating")
			cancel()
		case <-sigUsr2:
			SearchDepth = SearchDepth + 2
			log.Println("searching depth increased by 2")
		}
	}
	//Дополнительно: Ожидание всех горутин перед завершением

}
