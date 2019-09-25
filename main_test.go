package main

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/slok/kubewebhook/pkg/log"
)

func newNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func TestValidate(t *testing.T) {
	var testCases = []struct {
		regex        string
		max          int
		objects      []runtime.Object
		newNamespace metav1.Object
		valid        bool
	}{
		{
			"^t",
			2,
			[]runtime.Object{
				newNamespace("test"),
			},
			newNamespace("test2"),
			true,
		},
		{
			"^t",
			1,
			[]runtime.Object{
				newNamespace("test"),
			},
			newNamespace("test2"),
			false,
		},
		{
			"^t",
			5,
			[]runtime.Object{
				newNamespace("test"),
				newNamespace("test2"),
				newNamespace("test3"),
				newNamespace("test4"),
				newNamespace("test5"),
			},
			newNamespace("test6"),
			false,
		},
		{
			"^t",
			6,
			[]runtime.Object{
				newNamespace("test"),
				newNamespace("test2"),
				newNamespace("test3"),
				newNamespace("test4"),
				newNamespace("test5"),
			},
			newNamespace("test6"),
			true,
		},
		{
			".*",
			0,
			nil,
			newNamespace("test"),
			false,
		},
	}

	for _, tc := range testCases {

		rgx, err := regexp.Compile(tc.regex)
		if err != nil {
			t.Error(err)
		}

		nl := namespaceLimiter{
			namespaceRegex:      rgx,
			maxNumberNamespaces: tc.max,
			clientset:           fake.NewSimpleClientset(tc.objects...),
			logger:              &log.Std{Debug: true},
		}

		valid, _, _ := nl.Validate(context.Background(), tc.newNamespace)
		if tc.valid {
			assert.True(t, valid)
		} else {
			assert.False(t, valid)
		}
	}
}
