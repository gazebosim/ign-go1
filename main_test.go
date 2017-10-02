package ign

import (
  "log"
  "os"
  "testing"
)

// This function applies to ALL tests in the application.
// It will run the test and then clean the database.
func TestMain(m *testing.M) {
  code := m.Run()
  packageTearDown()
  log.Println("Cleaned database tables after all tests")
  os.Exit(code)
}

// Clean up our mess
func packageTearDown() {
  cleanDBTables()
}

func cleanDBTables() {
}
