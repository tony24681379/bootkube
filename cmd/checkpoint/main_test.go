package main

import (
	"fmt"
	"reflect"
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

func TestProcess(t *testing.T) {
	type testCase struct {
		desc                string
		localRunning        map[string]*v1.Pod
		localParents        map[string]*v1.Pod
		apiParents          map[string]*v1.Pod
		activeCheckpoints   map[string]*v1.Pod
		inactiveCheckpoints map[string]*v1.Pod
		expectStart         []string
		expectStop          []string
		expectRemove        []string
		podName             string
	}

	cases := []testCase{
		{
			desc:                "Inactive checkpoint and no local running: should start",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			expectStart:         []string{"AA"},
		},
		{
			desc:                "Inactive checkpoint and local running: no change",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:                "Inactive checkpoint and no api parent: should remove",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"BB": {}},
			expectRemove:        []string{"AA"},
		},
		{
			desc:                "Inactive checkpoint and both api & local running: no change",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:                "Inactive checkpoint and only api parent: should start",
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"AA": {}},
			expectStart:         []string{"AA"},
		},
		{
			desc:              "Active checkpoint and no local running: no change",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint and local running: should stop",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:      map[string]*v1.Pod{"AA": {}},
			expectStop:        []string{"AA"},
		},
		{
			desc:              "Active checkpoint and api parent: no change",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint and no api parent: remove",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"BB": {}},
			expectRemove:      []string{"AA"},
		},
		{
			desc:              "Active checkpoint with local running, and api parent: should stop",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:      map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"AA": {}},
			expectStop:        []string{"AA"},
		},
		{
			desc:              "Active checkpoint with local parent, and no api parent: should remove",
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localParents:      map[string]*v1.Pod{"AA": {}},
			apiParents:        map[string]*v1.Pod{"BB": {}},
			expectRemove:      []string{"AA"},
		},
		{
			desc:                "Both active and inactive checkpoints, with no api parent: remove both",
			activeCheckpoints:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			apiParents:          map[string]*v1.Pod{"BB": {}},
			expectRemove:        []string{"AA"}, // Only need single remove, we should clean up both active/inactive
		},
		{
			desc:                "Inactive checkpoint, local parent, local running, no api parent: no change", // Safety check - don't GC if local parent still exists (even if possibly stale)
			inactiveCheckpoints: map[string]*v1.Pod{"AA": {}},
			localRunning:        map[string]*v1.Pod{"AA": {}},
			localParents:        map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:              "Active checkpoint, local parent, no local running, no api parent: no change", // Safety check - don't GC if local parent still exists (even if possibly stale)
			activeCheckpoints: map[string]*v1.Pod{"AA": {}},
			localParents:      map[string]*v1.Pod{"AA": {}},
		},
		{
			desc:         "Inactive pod-checkpointer, local parent, local running, api parent: should start",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			apiParents:   map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Inactive pod-checkpointer, local parent, no local running, api not reachable: should start",
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Inactive pod-checkpointer, no local parent, no api parent: should remove in the last",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"BB": {}},
			apiParents:   map[string]*v1.Pod{"BB": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Inactive pod-checkpointer, no local parent, no api parent: should remove all",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Active pod-checkpointer, no local parent, no api parent: should remove all",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			activeCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, local parent, local running, api parent: should start",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			apiParents:   map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, local parent, no local running, api not reachable: should start",
			podName:      "pod-checkpointer-mynode",
			localParents: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
			},
			expectStart: []string{"kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, no local parent, no api parent: should remove in the last",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"BB": {}},
			apiParents:   map[string]*v1.Pod{"BB": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Inactive pod-checkpointer, no local parent, no api parent: should remove all",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			inactiveCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
		{
			desc:         "Running as an on-disk checkpointer: Active pod-checkpointer, no local parent, no api parent: should remove all",
			podName:      "pod-checkpointer-mynode",
			localRunning: map[string]*v1.Pod{"kube-system/pod-checkpointer": {}, "AA": {}},
			localParents: map[string]*v1.Pod{"AA": {}},
			apiParents:   map[string]*v1.Pod{"AA": {}},
			activeCheckpoints: map[string]*v1.Pod{
				"kube-system/pod-checkpointer": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "kube-system",
						Name:      "pod-checkpointer",
					},
				},
				"AA": {},
			},
			expectRemove: []string{"AA", "kube-system/pod-checkpointer"},
		},
	}

	for _, tc := range cases {
		nodeName, podName, podNamespace = "mynode", "pod-checkpointer", "kube-system"
		if tc.podName != "" {
			podName = tc.podName
		}
		gotStart, gotStop, gotRemove := process(tc.localRunning, tc.localParents, tc.apiParents, tc.activeCheckpoints, tc.inactiveCheckpoints)
		if !reflect.DeepEqual(tc.expectStart, gotStart) ||
			!reflect.DeepEqual(tc.expectStop, gotStop) ||
			!reflect.DeepEqual(tc.expectRemove, gotRemove) {
			t.Errorf("For test: %s\nExpected start: %s Got: %s\nExpected stop: %s Got: %s\nExpected remove: %s Got: %s\n",
				tc.desc, tc.expectStart, gotStart, tc.expectStop, gotStop, tc.expectRemove, gotRemove)
		}
	}
}

func TestSanitizeCheckpointPod(t *testing.T) {
	type testCase struct {
		desc     string
		pod      *v1.Pod
		expected *v1.Pod
	}
	trueVar := true

	cases := []testCase{
		{
			desc: "Pod name and namespace are preserved, checkpoint annotation added, owner points to parent",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "Existing annotations are removed, checkpoint annotation added, owner points to parent",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "podname",
					Namespace:   "podnamespace",
					Annotations: map[string]string{"foo": "bar"},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "Pod status is reset",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
				},
				Status: v1.PodStatus{Phase: "Pending"},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "Service acounts are cleared",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
				},
				Spec: v1.PodSpec{ServiceAccountName: "foo", DeprecatedServiceAccount: "bar"},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "Labels are preserved",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
					Labels:    map[string]string{"foo": "bar"},
				},
			},
			expected: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "podname",
					Namespace:       "podnamespace",
					Labels:          map[string]string{"foo": "bar"},
					Annotations:     map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{{Name: "podname", Controller: &trueVar}},
				},
			},
		},
		{
			desc: "OwnerReference of checkpoint points to parent pod",
			pod: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "podname",
					Namespace: "podnamespace",
					UID:       "pod-uid",
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "v1", Kind: "Daemonset", Name: "daemonname", UID: "daemon-uid"},
					},
				},
			},
			expected: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "podname",
					Namespace:   "podnamespace",
					Annotations: map[string]string{checkpointParentAnnotation: "podname"},
					OwnerReferences: []metav1.OwnerReference{
						{APIVersion: "v1", Kind: "Pod", Name: "podname", UID: "pod-uid", Controller: &trueVar},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		got, err := sanitizeCheckpointPod(tc.pod)
		if err != nil {
			t.Errorf("\nUnexpected error: %v\n", err)
		}
		if !apiequality.Semantic.DeepEqual(tc.expected, got) {
			t.Errorf("\nFor Test: %s\n\nExpected:\n%#v\nGot:\n%#v\n", tc.desc, tc.expected, got)
		}
	}

}

func TestPodListToParentPods(t *testing.T) {
	parentAPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "A", Namespace: "A", Annotations: map[string]string{shouldCheckpointAnnotation: "true"}}}
	parentBPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "B", Namespace: "B", Annotations: map[string]string{shouldCheckpointAnnotation: "true"}}}
	checkpointPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "C", Namespace: "C", Annotations: map[string]string{checkpointParentAnnotation: "foo/bar"}}}
	regularPod := v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "D", Namespace: "D", Annotations: map[string]string{"meta": "data"}}}

	type testCase struct {
		desc     string
		podList  *v1.PodList
		expected map[string]*v1.Pod
	}

	cases := []testCase{
		{
			desc:     "Both regular pods, none are parents",
			podList:  &v1.PodList{Items: []v1.Pod{regularPod, regularPod}},
			expected: nil,
		},
		{
			desc:     "Regular and checkpoint pods, none are parents",
			podList:  &v1.PodList{Items: []v1.Pod{regularPod, checkpointPod}},
			expected: nil,
		},
		{
			desc:     "One parent and one regular pod: Should return only parent",
			podList:  &v1.PodList{Items: []v1.Pod{parentAPod, regularPod}},
			expected: map[string]*v1.Pod{"A/A": {}},
		},
		{
			desc:     "Two parent pods, should return both",
			podList:  &v1.PodList{Items: []v1.Pod{parentAPod, parentBPod}},
			expected: map[string]*v1.Pod{"A/A": {}, "B/B": {}},
		},
	}

	for _, tc := range cases {
		got := podListToParentPods(tc.podList)
		if len(got) != len(tc.expected) {
			t.Errorf("Expected: %d pods but got %d for test: %s", len(tc.expected), len(got), tc.desc)
		}
		for e := range tc.expected {
			if _, ok := got[e]; !ok {
				t.Errorf("Missing expected PodFullName %s", e)
			}
		}
	}
}

func podWithAnnotations(a map[string]string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "podname",
			Namespace:   "podnamespace",
			Annotations: a,
		},
	}
}

func TestIsValidParent(t *testing.T) {
	type testCase struct {
		desc     string
		pod      *v1.Pod
		expected bool
	}

	cases := []testCase{
		{
			desc:     "Checkpoint pod",
			pod:      podWithAnnotations(map[string]string{checkpointParentAnnotation: "foo/bar"}),
			expected: false,
		},
		{
			desc:     "Static pod",
			pod:      podWithAnnotations(map[string]string{podSourceAnnotation: "file"}),
			expected: false,
		},
		{
			desc:     "Normal pod",
			pod:      podWithAnnotations(map[string]string{"foo": "bar"}),
			expected: false,
		},
		{
			desc:     "Parent pod",
			pod:      podWithAnnotations(map[string]string{shouldCheckpointAnnotation: "true"}),
			expected: true,
		},
		{
			desc:     "No annotation / normal pod",
			pod:      podWithAnnotations(nil),
			expected: false,
		},
		{
			desc: "Parent and static pod",
			pod: podWithAnnotations(map[string]string{
				shouldCheckpointAnnotation: "true",
				podSourceAnnotation:        "file",
			}),
			expected: false,
		},
		{
			desc: "Parent and checkpoint", // This should never happen
			pod: podWithAnnotations(map[string]string{
				shouldCheckpointAnnotation: "true",
				checkpointParentAnnotation: "foo/bar",
			}),
			expected: false,
		},
	}

	for _, tc := range cases {
		got := isValidParent(tc.pod)
		if tc.expected != got {
			t.Errorf("Expected: %t Got: %t For test: %s", tc.expected, got, tc.desc)
		}
	}
}

func TestIsCheckpoint(t *testing.T) {
	type testCase struct {
		desc     string
		pod      *v1.Pod
		expected bool
	}

	cases := []testCase{
		{
			desc: fmt.Sprintf("Pod is checkpoint and contains %s annotation key and value", checkpointParentAnnotation),
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{checkpointParentAnnotation: "podnamespace/podname"},
				},
			},
			expected: true,
		},
		{
			desc: fmt.Sprintf("Pod is checkpoint contains %s annotation key and no value", checkpointParentAnnotation),
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{checkpointParentAnnotation: ""},
				},
			},
			expected: true,
		},
		{
			desc: "Pod is not checkpoint & contains unrelated annotations",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"foo": "bar"},
				},
			},
			expected: false,
		},
		{
			desc: "Pod is not checkpoint & contains no annotations",
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: false,
		},
	}

	for _, tc := range cases {
		got := isCheckpoint(tc.pod)
		if tc.expected != got {
			t.Errorf("Expected: %t Got: %t for test: %s", tc.expected, got, tc.desc)
		}
	}
}

func TestCopyPod(t *testing.T) {
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podname",
			Namespace: "podnamespace",
		},
		Spec: v1.PodSpec{Containers: []v1.Container{{VolumeMounts: []v1.VolumeMount{{Name: "default-token"}}}}},
	}
	got, err := copyPod(&pod)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !apiequality.Semantic.DeepEqual(pod, *got) {
		t.Errorf("Expected:\n%#v\nGot:\n%#v", pod, got)
	}
}

func TestPodID(t *testing.T) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "podname",
			Namespace: "podnamespace",
		},
	}
	expected := "podnamespace/podname"
	got := PodFullName(pod)
	if expected != got {
		t.Errorf("Expected: %s Got: %s", expected, got)
	}
}

func TestPodIDToInactiveCheckpointPath(t *testing.T) {
	id := "foo/bar"
	expected := inactiveCheckpointPath + "/foo-bar.json"
	got := PodFullNameToInactiveCheckpointPath(id)
	if expected != got {
		t.Errorf("Expected: %s Got: %s", expected, got)
	}
}

func TestPodIDToActiveCheckpointPath(t *testing.T) {
	id := "foo/bar"
	expected := activeCheckpointPath + "/foo-bar.json"
	got := PodFullNameToActiveCheckpointPath(id)
	if expected != got {
		t.Errorf("Expected: %s Got: %s", expected, got)
	}
}

func TestPodIDToSecretPath(t *testing.T) {
	id := "foo/bar"
	expected := checkpointSecretPath + "/foo/bar"
	got := PodFullNameToSecretPath(id)
	if expected != got {
		t.Errorf("Expected %s Got %s", expected, got)
	}
}

func TestPodIDToConfigMapPath(t *testing.T) {
	id := "foo/bar"
	expected := checkpointConfigMapPath + "/foo/bar"
	got := PodFullNameToConfigMapPath(id)
	if expected != got {
		t.Errorf("Expected %s Got %s", expected, got)
	}
}
