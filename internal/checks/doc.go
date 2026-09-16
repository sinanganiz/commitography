// Package checks holds the repository checkers required by ADR-0055 clause 2
// and listed in ADR-0056 clause 1 table 2. Each checker is one test in this
// package, and every failure message begins with the record it enforces, in
// the form "ADR-NNNN: <what is wrong>" (ADR-0055 clause 3).
//
// Checkers read tracked files only, never the working tree as a whole:
// generated fixture repositories are untracked and deliberately not valid
// source.
//
// This package may import any package it verifies, and nothing may import it
// (ADR-0060 clause 2).
package checks
