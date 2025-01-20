package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

func parse_ls_tree(cmd *exec.Cmd) ([]string, error) {
	var output bytes.Buffer
	cmd.Stdout = &output
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	result := strings.Split(strings.TrimSpace(output.String()), "\n")
	return result, nil
}

func get_paths(repo_path, revision string) ([]string, error) {
	err := os.Chdir(repo_path)
	if err != nil {
		fmt.Printf("Failed to change directory: %v", err)
		return nil, err
	}
	cmd := exec.Command("git", "ls-tree", "--name-only", "-r", revision)
	return parse_ls_tree(cmd)

}

func reduse(i_result, result map[string]int) {
	for name, val := range i_result {
		if _, ok := result[name]; !ok {
			result[name] = val
			continue
		}
		result[name] += val
	}
}

func free_sema(sema chan struct{}) {
	<-sema
}

func calc_fame(path, repo_path, revision string, sema chan struct{}, results chan map[string]int) {
	defer free_sema(sema)
	cmd := exec.Command("git", "-C", repo_path, "blame", "--line-porcelain", revision, "--", path)
	var output bytes.Buffer
	cmd.Stdout = &output

	err := cmd.Run()
	if err != nil {
		fmt.Printf("failed to execute git blame: %v", err.Error())
		return
	}

	lines := strings.Split(output.String(), "\n")
	authorLineCount := make(map[string]int)

	for _, line := range lines {
		if strings.HasPrefix(line, "author ") {
			author := strings.TrimPrefix(line, "author ")
			authorLineCount[author]++
		}
	}
	select {
	case results <- authorLineCount:
		return
	}
}

func draw_progress_animation(done chan struct{}) {
	image_idx := 0
	image := [2]string{"(.)(.)", "(*)(*)"}
	for {
		select {
		case <-done:
			return
		default:
			fmt.Printf("\r%v", image[image_idx])
			image_idx++
			image_idx %= 2
			time.Sleep(time.Second)
		}
	}
}

func main() {
	repo_path := flag.String("repo", ".", "path to repository wich been calcilated")
	revision := flag.String("rev", "HEAD", "hash of commit which been calculated")
	flag.Parse()
	fmt.Printf("%v - path to repo, %v hash of commit \n", *repo_path, *revision)
	paths, err := get_paths(*repo_path, *revision)
	if err != nil {
		fmt.Printf("sosal? %v", err.Error())
		return
	}
	sema := make(chan struct{}, 20)
	results := make(chan map[string]int, 20)
	result := make(map[string]int)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case i_result := <-results:
				reduse(i_result, result)
			case <-done:
				return
			}
		}
	}()
	var wg sync.WaitGroup
	go draw_progress_animation(done)
	for _, path := range paths {
		select {
		case sema <- struct{}{}:
			wg.Add(1)
			go func() {
				defer wg.Done()
				calc_fame(path, *repo_path, *revision, sema, results)
			}()
		}
	}
	wg.Wait()
	close(done)
	fmt.Print("\n")
	for name, val := range result {
		fmt.Printf("%v commited %v strings\n", name, val)
	}
}
