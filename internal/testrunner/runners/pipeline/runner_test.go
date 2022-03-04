// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

const (
	firstTestResult  = "first"
	secondTestResult = "second"
	thirdTestResult  = "third"

	emptyTestResult = ""
)

func TestStripEmptyTestResults(t *testing.T) {
	given := &testResult{
		events: []json.RawMessage{
			[]byte(firstTestResult),
			nil,
			nil,
			[]byte(emptyTestResult),
			[]byte(secondTestResult),
			nil,
			[]byte(thirdTestResult),
			nil,
		},
	}

	actual := stripEmptyTestResults(given)
	require.Len(t, actual.events, 4)
	require.Equal(t, actual.events[0], json.RawMessage(firstTestResult))
	require.Equal(t, actual.events[1], json.RawMessage(emptyTestResult))
	require.Equal(t, actual.events[2], json.RawMessage(secondTestResult))
	require.Equal(t, actual.events[3], json.RawMessage(thirdTestResult))
}

var diffUliteTests = []struct {
	name string
	a, b string
	u    int
	want string
}{
	{
		name: "no diff",
		u:    3,
		a: `a
b
c
d
`,
		b: `a
b
c
d
`,
		want: "",
	},
	{
		name: "first line",
		u:    3,
		a: `a change
b
c
d
e
`,
		b: `a
b
c
d
e
`,
		want: `--- want
+++ got
@@ -1 +1 @@
- a change
+ a
  b
  c
  d`,
	},
	{
		name: "last line",
		u:    3,
		a: `a
b
c
d
e change
`,
		b: `a
b
c
d
e
`,
		want: `--- want
+++ got
@@ -2 +2 @@
  b
  c
  d
- e change
+ e
  `,
	},
	{
		name: "middle",
		u:    3,
		a: `a
b
c
d change
e
f
g
h
`,
		b: `a
b
c
d
e
f
g
h
`,
		want: `--- want
+++ got
@@ -1 +1 @@
  a
  b
  c
- d change
+ d
  e
  f
  g`,
	},
	{
		name: "close pair",
		u:    3,
		a: `a
b
c
d change
e
f
g
h
i
j
k
l
m
`,
		b: `a
b
c
d
e
f
g
h
i change
j
k
l
m
`,
		want: `--- want
+++ got
@@ -1 +1 @@
  a
  b
  c
- d change
+ d
  e
  f
  g
  h
- i
+ i change
  j
  k
  l`,
	},
	{
		name: "far pair",
		u:    3,
		a: `a
b
c change
d
e
f
g
h
i
j
k
l
m
`,
		b: `a
b
c
d
e
f
g
h
i
j
k
l change
m
`,
		want: `--- want
+++ got
@@ -1 +1 @@
  a
  b
- c change
+ c
  d
  e
  f
@@ -9 +9 @@
  i
  j
  k
- l
+ l change
  m
  `,
	},
	{
		name: "far pair addition",
		u:    3,
		a: `a
b
c change
d
e
f
g
a
b
c
d
e
f
g
h
i
j
k
l
m
`,
		b: `a
b
c
d
e
f
g
h
i
j
k
l change
m
`,
		want: `--- want
+++ got
@@ -1 +1 @@
  a
  b
- c change
- d
- e
- f
- g
- a
- b
  c
  d
  e
@@ -16 +9 @@
  i
  j
  k
- l
+ l change
  m
  `,
	},
}

func TestDiffUlite(t *testing.T) {
	for _, test := range diffUliteTests {
		t.Run(test.name, func(t *testing.T) {
			got := diffUlite(test.a, test.b, test.u)
			if got != test.want {
				t.Errorf("unexpected result\n%s", cmp.Diff(got, test.want))
			}
		})
	}
}
