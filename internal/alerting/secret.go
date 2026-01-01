package alerting

import (
	"context"
	"fmt"

	"github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getValueFromSecret(ctx context.Context, client client.Client, secretRef v1alpha1.NamespacedSecretKeyRef) (string, error) {
	secret := &corev1.Secret{}
	err := client.Get(
		ctx, types.NamespacedName{
			Namespace: secretRef.Namespace,
			Name:      secretRef.Name,
		}, secret,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	value, ok := secret.Data[secretRef.Key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret", secretRef.Key)
	}

	return string(value), nil
}
