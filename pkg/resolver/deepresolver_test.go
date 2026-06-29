// Copyright (c) 2017-2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package resolver

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"daml.com/x/assistant/pkg/assembler"
	"daml.com/x/assistant/pkg/assistantconfig"
	"daml.com/x/assistant/pkg/packagelock"
	"daml.com/x/assistant/pkg/resolution"
	"daml.com/x/assistant/pkg/sdkmanifest"
	"daml.com/x/assistant/pkg/testutil"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSinglePackage(t *testing.T) {
	ctx := testutil.Context(t)

	packagePath := testutil.TestdataPath(t, "daml-package", testutil.OS)
	t.Setenv(assistantconfig.EditionEnvVar, "enterprise")
	withResolver(t, packagePath, func(resolver *DeepResolver) {
		result, err := resolver.RunDeepResolution(ctx)
		require.NoError(t, err)

		assert.Len(t, result.Packages, 1)
		assert.Contains(t, lo.Keys(result.Packages)[0], packagePath)
		assert.Len(t, lo.Values(result.Packages)[0].Components, 2)
		assert.Len(t, lo.Values(result.Packages)[0].Imports, 4)
		assert.Equal(t, resolution.Kind, result.Kind)
		assert.Equal(t, resolution.ApiVersion, result.APIVersion)
	})
}

func TestMultiPackage(t *testing.T) {
	ctx := testutil.Context(t)

	multiPackagePath := testutil.TestdataPath(t, "multi-package", testutil.OS)
	packagePath := testutil.TestdataPath(t, "daml-package", testutil.OS)
	t.Setenv(assistantconfig.EditionEnvVar, "enterprise")
	t.Setenv(assistantconfig.DamlMultiPackageEnvVar, multiPackagePath)

	withResolver(t, packagePath, func(resolver *DeepResolver) {
		result, err := resolver.RunDeepResolution(ctx)
		require.NoError(t, err)
		assert.Len(t, result.Packages, 1)
		assert.Contains(t, lo.Keys(result.Packages)[0], packagePath)
		assert.Len(t, lo.Values(result.Packages)[0].Components, 3)
		assert.Len(t, lo.Values(result.Packages)[0].Imports, 4)
		assert.Equal(t, resolution.Kind, result.Kind)
		assert.Equal(t, resolution.ApiVersion, result.APIVersion)
	})
}

func TestResolvePathExpandsLockfilePathRelativeToDpmHome(t *testing.T) {
	dpmHome := filepath.Join(string(filepath.Separator), "tmp", "dpm-home")
	resolver := &DeepResolver{
		config: &assistantconfig.Config{DamlHomePath: dpmHome},
	}
	ociURI, err := url.Parse("oci://example.com/foo:1.2.3")
	require.NoError(t, err)

	got := resolver.resolveDarPath(&packagelock.Dar{
		URI:  ociURI,
		Path: "${DPM_HOME}/cache/dars/abc/foo.dar",
	})

	assert.Equal(t, filepath.Join(dpmHome, "cache", "dars", "abc", "foo.dar"), got)
}

func TestResolvePathExpandsBareRelativeLockfilePathForCompatibility(t *testing.T) {
	dpmHome := filepath.Join(string(filepath.Separator), "tmp", "dpm-home")
	resolver := &DeepResolver{
		config: &assistantconfig.Config{DamlHomePath: dpmHome},
	}
	ociURI, err := url.Parse("oci://example.com/foo:1.2.3")
	require.NoError(t, err)

	got := resolver.resolveDarPath(&packagelock.Dar{
		URI:  ociURI,
		Path: "cache/dars/abc/foo.dar",
	})

	assert.Equal(t, filepath.Join(dpmHome, "cache", "dars", "abc", "foo.dar"), got)
}

func TestResolvePathLeavesBuiltinLockfilePathRelative(t *testing.T) {
	resolver := &DeepResolver{
		config: &assistantconfig.Config{DamlHomePath: filepath.Join(string(filepath.Separator), "tmp", "dpm-home")},
	}
	builtinURI, err := url.Parse("builtin://daml-prim")
	require.NoError(t, err)

	got := resolver.resolveDarPath(&packagelock.Dar{
		URI:  builtinURI,
		Path: "daml-prim",
	})

	assert.Equal(t, "daml-prim", got)
}

func withResolver(t *testing.T, damlPackagePath string, testFn func(*DeepResolver)) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// reset original cwd
	t.Cleanup(func() { require.NoError(t, os.Chdir(cwd)) })

	// this will make daml.yaml in the CWD
	require.NoError(t, os.Chdir(damlPackagePath))

	config := &assistantconfig.Config{
		Edition:                   assistantconfig.NewLazyEdition(sdkmanifest.OpenSource),
		InstalledSdkManifestsPath: testutil.TestdataPath(t, "installed-sdks"),
	}

	a, err := assembler.Fake(nil)
	require.NoError(t, err)

	resolver := New(config, a)
	testFn(resolver)
}
