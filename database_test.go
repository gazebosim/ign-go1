package ign

import (
  "testing"
)

/////////////////////////////////////////////////
// Test a bad connection to the database
func TestBadDatabase(t *testing.T) {
  db, err := DBInit("bad", "bad", "bad", "bad")

  if err == nil {
    t.Fatal("Should have received an error from the database")
  }

  if db != nil {
    t.Fatal("Database should be nil")
  }
}

/// \todo: Figure out how to test the database without including username
/// and password information in the source code
