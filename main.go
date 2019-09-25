package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	whhttp "github.com/slok/kubewebhook/pkg/http"
	"github.com/slok/kubewebhook/pkg/log"
	validatingwh "github.com/slok/kubewebhook/pkg/webhook/validating"
)

type namespaceLimiter struct {
	namespaceRegex      *regexp.Regexp
	maxNumberNamespaces int
	clientset           kubernetes.Interface

	logger log.Logger
}

func (nl *namespaceLimiter) Validate(_ context.Context, obj metav1.Object) (bool, validatingwh.ValidatorResult, error) {
	namespace, ok := obj.(*corev1.Namespace)

	if !ok {
		return false, validatingwh.ValidatorResult{}, fmt.Errorf("not a namespace")
	}

	namespaces, err := nl.clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return false, validatingwh.ValidatorResult{}, err
	}

	var matchingNamespaces int
	for _, ns := range namespaces.Items {
		if nl.namespaceRegex.MatchString(ns.Name) {
			matchingNamespaces++
		}
	}

	if matchingNamespaces >= nl.maxNumberNamespaces {
		nl.logger.Infof("namespace %s denied, currently %d namespaces match the regex", namespace.Name, matchingNamespaces)
		res := validatingwh.ValidatorResult{
			Valid:   false,
			Message: fmt.Sprintf("too many (%d) namespaces matching regex. %s namespace denied", matchingNamespaces, namespace.Name),
		}
		return false, res, nil
	}

	nl.logger.Infof("namespace %s is valid", namespace.Name)
	res := validatingwh.ValidatorResult{
		Valid:   true,
		Message: "namespace is valid",
	}
	return true, res, nil
}

type config struct {
	certFile string
	keyFile  string
	addr     string

	namespaceRegex      string
	maxNumberNamespaces int
}

func initFlags() *config {
	cfg := &config{}

	fl := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fl.StringVar(&cfg.certFile, "tls-cert-file", "", "TLS certificate file")
	fl.StringVar(&cfg.keyFile, "tls-key-file", "", "TLS key file")
	fl.StringVar(&cfg.addr, "listen-addr", ":8080", "The address to start the server")
	fl.StringVar(&cfg.namespaceRegex, "namespace-regex", "", "The namespace name regex that matches namespaces that should be limited")
	fl.IntVar(&cfg.maxNumberNamespaces, "namespace-max", 0, "The maximum number of namespaces matching the regex that should be allowed")

	fl.Parse(os.Args[1:])
	return cfg
}

func main() {
	logger := &log.Std{Debug: true}

	cfg := initFlags()

	// Create our validator
	rgx, err := regexp.Compile(cfg.namespaceRegex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regex: %s", err)
		os.Exit(1)
		return
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get in-cluster-config: %s", err)
		os.Exit(1)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create kubernetes clientset: %s", err)
		os.Exit(1)
	}

	vl := &namespaceLimiter{
		namespaceRegex:      rgx,
		maxNumberNamespaces: cfg.maxNumberNamespaces,
		logger:              logger,
		clientset:           clientset,
	}

	vcfg := validatingwh.WebhookConfig{
		Name: "namespaceLimiter",
		Obj:  &corev1.Namespace{},
	}
	wh, err := validatingwh.NewWebhook(vcfg, vl, nil, nil, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating webhook: %s", err)
		os.Exit(1)
	}

	// Serve the webhook.
	logger.Infof("Listening on %s", cfg.addr)
	err = http.ListenAndServeTLS(cfg.addr, cfg.certFile, cfg.keyFile, whhttp.MustHandlerFor(wh))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error serving webhook: %s", err)
		os.Exit(1)
	}
}
