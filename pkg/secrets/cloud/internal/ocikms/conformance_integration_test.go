//go:build ociintegration

// Copyright 2026, Pulumi Corporation.
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

package ocikms_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"gocloud.dev/gcerrors"
	"gocloud.dev/secrets"
	"gocloud.dev/secrets/driver"
	"gocloud.dev/secrets/drivertest"
)

type publicDriver struct{ *secrets.Keeper }

func (publicDriver) ErrorCode(err error) gcerrors.ErrorCode { return gcerrors.Code(err) }

type realHarness struct{ urls [2]string }

func (h realHarness) Close() {}
func (h realHarness) MakeDriver(ctx context.Context) (driver.Keeper, driver.Keeper, error) {
	first, err := secrets.OpenKeeper(ctx, h.urls[0])
	if err != nil {
		return nil, nil, err
	}
	second, err := secrets.OpenKeeper(ctx, h.urls[1])
	if err != nil {
		first.Close()
		return nil, nil, err
	}
	return publicDriver{first}, publicDriver{second}, nil
}

func TestOCIConformance(t *testing.T) {
	drivertest.RunConformanceTests(t, func(ctx context.Context, t *testing.T) (drivertest.Harness, error) {
		urls := [2]string{os.Getenv("OCIKMS_TEST_URL"), os.Getenv("OCIKMS_SECOND_TEST_URL")}
		if urls[0] == "" || urls[1] == "" || urls[0] == urls[1] {
			return nil, fmt.Errorf("two distinct authorized OCI test key URLs are required")
		}
		return realHarness{urls: urls}, nil
	}, nil)
}
