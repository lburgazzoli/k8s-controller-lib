// Copyright 2025 The k8s-controller-lib Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mocks_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util/test/mocks"

	. "github.com/onsi/gomega"
)

func TestController_MockBasics(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := mocks.NewController()
	mockCtrl.On("Watch", mock.Anything).Return(nil)

	// Verify we can call Watch
	err := mockCtrl.Watch(nil)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify expectations using testify's built-in methods
	mockCtrl.AssertCalled(t, "Watch", mock.Anything)
	mockCtrl.AssertNumberOfCalls(t, "Watch", 1)
}

func TestController_ErrorSimulation(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := mocks.NewController()

	// Configure to return an error
	expectedErr := reconcile.TerminalError(nil)
	mockCtrl.On("Watch", mock.Anything).Return(expectedErr)

	err := mockCtrl.Watch(nil)
	g.Expect(err).To(Equal(expectedErr))
}
