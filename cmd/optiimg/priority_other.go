//go:build !darwin && !linux && !windows

package main

func LowerPriority() error { return nil }
