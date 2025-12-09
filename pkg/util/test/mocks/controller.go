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

package mocks

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Controller is a mock implementation of controller.Controller for testing.
// Use testify's .Run() method to capture Watch calls in your tests:
//
//	var watchedSources []source.Source
//	mockCtrl.On("Watch", mock.Anything).Run(func(args mock.Arguments) {
//	    watchedSources = append(watchedSources, args.Get(0).(source.Source))
//	}).Return(nil)
type Controller struct {
	mock.Mock
}

// NewController creates a new Controller mock for testing.
func NewController() *Controller {
	return &Controller{}
}

// Reconcile implements controller.Controller.
func (m *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	args := m.Called(ctx, req)

	return args.Get(0).(reconcile.Result), args.Error(1)
}

// Watch implements controller.Controller.
// Use testify's AssertNumberOfCalls to verify call count in tests.
func (m *Controller) Watch(src source.Source) error {
	args := m.Called(src)

	return args.Error(0)
}

// Start implements controller.Controller.
func (m *Controller) Start(ctx context.Context) error {
	args := m.Called(ctx)

	return args.Error(0)
}

// GetLogger implements controller.Controller.
func (m *Controller) GetLogger() logr.Logger {
	args := m.Called()
	if len(args) > 0 && args.Get(0) != nil {
		return args.Get(0).(logr.Logger)
	}

	return ctrl.Log
}

// NeedLeaderElection implements controller.Controller.
func (m *Controller) NeedLeaderElection() bool {
	args := m.Called()
	if len(args) > 0 {
		return args.Bool(0)
	}

	return false
}

// Ensure Controller implements controller.Controller.
var _ controller.Controller = (*Controller)(nil)
