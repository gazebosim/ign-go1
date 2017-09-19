package igngo

import (
  "archive/zip"
  "bytes"
  "errors"
  "fmt"
  "io"
  "math/rand"
  "os"
  "path/filepath"
  "runtime"
  "strconv"
  "time"
)

// Read an environment variable and return an error if not present
func ReadEnvVar(name string) (string, error) {
  value := os.Getenv(name)
  if value == "" {
    return "", errors.New("Missing " + name + " env variable.")
  }
  return value, nil
}

// Unzip a memory buffer
func Unzip(buff bytes.Buffer, size int64, dest string, verbose bool) error {
  reader, err := zip.NewReader(bytes.NewReader(buff.Bytes()), size)
  if err != nil {
    return errors.New("unzip: Unable to read byte buffer")
  }
  return UnzipImpl(reader, dest, verbose)
}

// unzip extracts a compressed .zip file
func UnzipFile(zipfile string, dest string, verbose bool) error {
  reader, err := zip.OpenReader(zipfile)
  if err != nil {
    return errors.New("unzip: Unable to open [" + zipfile + "]")
  }
  defer reader.Close()
  return UnzipImpl(&reader.Reader, dest, verbose)
}

// Helper unzip implementation
func UnzipImpl(reader *zip.Reader, dest string, verbose bool) error {
  for _, f := range reader.File {
    zipped, err := f.Open()
    if err != nil {
      return errors.New("unzip: Unable to open [" + f.Name + "]")
    }

    defer zipped.Close()

    path := filepath.Join(dest, f.Name)
    if f.FileInfo().IsDir() {
      os.MkdirAll(path, f.Mode())
      if verbose {
        fmt.Println("Creating directory", path)
      }
    } else {
      // Ensure we create the parent folder
      err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
      if err != nil {
        return errors.New("unzip: Unable to create parent folder [" + path + "]")
      }

      writer, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, f.Mode())
      if err != nil {
        return errors.New("unzip: Unable to create [" + path + "]")
      }

      defer writer.Close()

      if _, err = io.Copy(writer, zipped); err != nil {
        return errors.New("unzip: Unable to create content in [" + path + "]")
      }

      if verbose {
        fmt.Println("Decompressing : ", path)
      }
    }
  }
  return nil
}

// Trace returns the filename, line and function name of its caller.
// Ref: http://stackoverflow.com/questions/25927660/golang-get-current-scope-of-function-name
func Trace() (string) {
  // At least one entry needed
  pc := make([]uintptr, 10)
  runtime.Callers(3, pc)
  f := runtime.FuncForPC(pc[0])
  file, line := f.FileLine(pc[0])
  return filepath.Base(file) + ":" + strconv.Itoa(line) + " " + f.Name()
}

// RandomString creates a random string of a given length.
// Ref: https://siongui.github.io/2015/04/13/go-generate-random-string/
func RandomString(strlen int) string {
  rand.Seed(time.Now().UTC().UnixNano())
  const chars = "abcdefghijklmnopqrstuvwxyz"
  result := make([]byte, strlen)
  for i := 0; i < strlen; i++ {
    result[i] = chars[rand.Intn(len(chars))]
  }
  return string(result)
}

// Min is an implementation of "int" Min
// See https://mrekucci.blogspot.com.ar/2015/07/dont-abuse-mathmax-mathmin.html
func Min(x, y int64) int64 {
  if x < y {
    return x
  }
  return y
}

// Max is an implementation of "int" Max
// See https://mrekucci.blogspot.com.ar/2015/07/dont-abuse-mathmax-mathmin.html
func Max(x, y int64) int64 {
  if x > y {
    return x
  }
  return y
}
