/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
)

// NewStore creates a store based on configuration
func NewStore(config *v1alpha1.StorageConfig) (Store, error) {
	if config == nil {
		// Default to SQLite
		return NewSQLiteStore("/data/guardian.db"), nil
	}

	switch config.Type {
	case "sqlite", "":
		path := "/data/guardian.db"
		if config.SQLite != nil && config.SQLite.Path != "" {
			path = config.SQLite.Path
		}
		return NewSQLiteStore(path), nil

	case "postgres":
		if config.PostgreSQL == nil {
			return nil, fmt.Errorf("postgres config required when type is postgres")
		}
		pg := config.PostgreSQL
		port := int32(5432)
		if pg.Port != nil {
			port = *pg.Port
		}
		// Note: credentials should be resolved by caller using NewStoreWithCredentials
		return NewPostgresStore(pg.Host, port, pg.Database, "", "", pg.SSLMode), nil

	case "mysql":
		if config.MySQL == nil {
			return nil, fmt.Errorf("mysql config required when type is mysql")
		}
		my := config.MySQL
		port := int32(3306)
		if my.Port != nil {
			port = *my.Port
		}
		// Note: credentials should be resolved by caller using NewStoreWithCredentials
		return NewMySQLStore(my.Host, port, my.Database, "", ""), nil

	default:
		return nil, fmt.Errorf("unknown storage type: %s", config.Type)
	}
}

// NewStoreWithCredentials creates a store and resolves credentials from secrets
func NewStoreWithCredentials(ctx context.Context, c client.Client, config *v1alpha1.StorageConfig) (Store, error) {
	if config == nil {
		// Default to SQLite
		return NewSQLiteStore("/data/guardian.db"), nil
	}

	switch config.Type {
	case "sqlite", "":
		path := "/data/guardian.db"
		if config.SQLite != nil && config.SQLite.Path != "" {
			path = config.SQLite.Path
		}
		return NewSQLiteStore(path), nil

	case "postgres":
		if config.PostgreSQL == nil {
			return nil, fmt.Errorf("postgres config required when type is postgres")
		}
		pg := config.PostgreSQL
		port := int32(5432)
		if pg.Port != nil {
			port = *pg.Port
		}

		// Resolve credentials from secret
		user, password, err := getCredentialsFromSecret(ctx, c, pg.CredentialsSecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get postgres credentials: %w", err)
		}

		return NewPostgresStore(pg.Host, port, pg.Database, user, password, pg.SSLMode), nil

	case "mysql":
		if config.MySQL == nil {
			return nil, fmt.Errorf("mysql config required when type is mysql")
		}
		my := config.MySQL
		port := int32(3306)
		if my.Port != nil {
			port = *my.Port
		}

		// Resolve credentials from secret
		user, password, err := getCredentialsFromSecret(ctx, c, my.CredentialsSecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get mysql credentials: %w", err)
		}

		return NewMySQLStore(my.Host, port, my.Database, user, password), nil

	default:
		return nil, fmt.Errorf("unknown storage type: %s", config.Type)
	}
}

// getCredentialsFromSecret retrieves username and password from a secret
func getCredentialsFromSecret(ctx context.Context, c client.Client, ref v1alpha1.NamespacedSecretRef) (string, string, error) {
	secret := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}, secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to get secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}

	username, ok := secret.Data["username"]
	if !ok {
		return "", "", fmt.Errorf("secret %s/%s missing 'username' key", ref.Namespace, ref.Name)
	}

	password, ok := secret.Data["password"]
	if !ok {
		return "", "", fmt.Errorf("secret %s/%s missing 'password' key", ref.Namespace, ref.Name)
	}

	return string(username), string(password), nil
}
