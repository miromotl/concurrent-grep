// An implementation of a concurrent grep command line tool
// It uses go routines, channels and regular expressions

package main

import (
    "fmt"
    "runtime"
    "os"
    "path/filepath"
    "log"
    "regexp"
    "bufio"
    "bytes"
    "io"
)

// We use as many go routines as workes as there are cores/processors
// in the computer.
var cntWorkers = runtime.NumCPU()

// The Result struct that is returned with every match of the regexp
type Result struct {
    fname string
    lino  int
    line  string
}

// The Job struct holds the filename and the result channel
// of the current job
type Job struct {
    fname   string
    results chan<- Result
}

// Do does the job for one file: matches the regex for each line
// and returns the result in an channel.
func (job Job) Do(lineRx *regexp.Regexp) {
    file, err := os.Open(job.fname)
    if err != nil {
        log.Printf("error: %s\n", err)
    }
    defer file.Close()

    reader := bufio.NewReader(file)
    for lino := 1; ; lino++ {
        line, err := reader.ReadBytes('\n')
        line = bytes.TrimRight(line, "\n\r")
        
        if lineRx.Match(line) {
            job.results <- Result{job.fname, lino, string(line)}
        }

        if err != nil {
            // Normally, we have reached EOF here
            if err != io.EOF {
                log.Printf("error: %d: %s\n", err)
            }
            break
        }
    }
}

// grep organizes the work:
// Creates the worker jobs, the communication channels
// and sets the whole machine to work
func grep(lineRx *regexp.Regexp, fnames []string) {
    // jobs channel is used for passing on jobs
    jobs := make(chan Job, cntWorkers)
    // results channel is used for collecting results
    results := make(chan Result, len(fnames))
    // done channel is used for signaling that a worker is done with its job
    done := make(chan struct{}, cntWorkers)

    // Each file is a job to do.
    // Add a Job struct to the jobs channel for each file, 
    // and then close the channel.
    go func() {
        for _, fname := range fnames {
            jobs <- Job{fname, results}
        }
        close(jobs)
    }()

    // Setup the worker goroutines that process
    // the jobs channel
    for i := 0; i < cntWorkers; i++ {
        go func() {
            for job := range jobs {
                job.Do(lineRx)
            }
            // jobs channel has been closed:
            // Signal that work has been done
            done <- struct{}{}
        }()
    }

    // Wait for the completion of all worker goroutines, and
    // then close the results channel
    go func() {
        for i := 0; i < cntWorkers; i++ {
            <-done
        }
        close(results)
    }()

    // Process the results in the main goroutine, reading from
    // the results channel until it is have been closed
    for result := range results {
        fmt.Printf("%s:%d:%s\n", result.fname, result.lino, result.line)
    }
}

// commandLineFiles globs the files in a Windows environement, otherwise
// it doesn't do anything
func commandLineFiles(fnames []string) []string {
    if runtime.GOOS == "windows" {
        args := make([]string, 0, len(fnames))

        for _, fname := range fnames {
            if matches, err := filepath.Glob(fname); err != nil {
                // not a valid pattern
                args = append(args, fname)
            } else if matches != nil {
                // at least one match
                args = append(args, matches...)
            }
        }

        return args
    }

    // not a Windows OS
    return fnames
}

func main() {
    runtime.GOMAXPROCS(runtime.NumCPU()) // Use all the machine's cores

    // Print usage string, if needed
    if len(os.Args) < 3 || os.Args[1] == "-h" || os.Args[1] == "--help" {
        fmt.Printf("usage: %s <regexp> <files>\n", filepath.Base(os.Args[0]))
        os.Exit(1)
    }

    // Compile the regular expression, on success call grep
    if lineRx, err := regexp.Compile(os.Args[1]); err != nil {
        log.Fatalf("invalid regexp: %s\n", err)
    } else {
        grep(lineRx, commandLineFiles(os.Args[2:]))
    }
}