package lari_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/dio/lari"
)

func ExampleGroup_Add_basic() {
	var g lari.Group
	{
		cancel := make(chan struct{})
		g.Add(func(context.Context) error {
			select {
			case <-time.After(time.Second):
				fmt.Printf("The first actor had its time elapsed\n")
				return nil
			case <-cancel:
				fmt.Printf("The first actor was canceled\n")
				return nil
			}
		}, func(err error) {
			fmt.Printf("The first actor was interrupted with: %v\n", err)
			close(cancel)
		})
	}
	{
		g.Add(func(context.Context) error {
			fmt.Printf("The second actor is returning immediately\n")
			return errors.New("immediate teardown")
		}, func(err error) {
			// Note that this interrupt function is called, even though the
			// corresponding execute function has already returned.
			fmt.Printf("The second actor was interrupted with: %v\n", err)
		})
	}
	fmt.Printf("The group was terminated with: %v\n", g.Run())
	// Output:
	// The second actor is returning immediately
	// The first actor was interrupted with: immediate teardown
	// The second actor was interrupted with: immediate teardown
	// The first actor was canceled
	// The group was terminated with: immediate teardown
}

func ExampleGroup_Add_context() {
	ctx, cancel := context.WithCancel(context.Background())
	var g lari.Group
	{
		ctx, cancel := context.WithCancel(ctx) // note: shadowed
		g.Add(func(context.Context) error {
			return runUntilCanceled(ctx)
		}, func(error) {
			cancel()
		})
	}
	go cancel()
	fmt.Printf("The group was terminated with: %v\n", g.Run())
	// Output:
	// The group was terminated with: context canceled
}

func ExampleGroup_Add_listener() {
	var g lari.Group
	{
		ln, _ := net.Listen("tcp", ":0")
		g.Add(func(context.Context) error {
			defer fmt.Printf("http.Serve returned\n")
			return http.Serve(ln, http.NewServeMux())
		}, func(error) {
			ln.Close()
		})
	}
	{
		g.Add(func(context.Context) error {
			return errors.New("immediate teardown")
		}, func(error) {
			//
		})
	}
	fmt.Printf("The group was terminated with: %v\n", g.Run())
	// Output:
	// http.Serve returned
	// The group was terminated with: immediate teardown
}

func runUntilCanceled(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
