# Credential

## Overview

The `credential` is used to store user credentials, like password, token, app secret, etc.

```yaml
      password:
        valueFrom:
          secretKeyRef:
            key: secret
            name: global-receiver-secret
            namespace: kubesphere-monitoring-system  
```

```yaml
      password:
        value: 123456
```

A `credential` allows the user to define:

- `value` - The value saved in plaintext, not recommended for storing confidential information.
- `valueFrom` - The object used to store user credentials, now only support `secret`.
- `valueFrom.secretKeyRef` - The secret used to store user credentials.
- `valueFrom.secretKeyRef.name` - The name of secret used to store user credentials.
- `valueFrom.secretKeyRef.namespace` - The namespace of secret used to store user credentials.
- `valueFrom.secretKeyRef.key` - The key of secret used to store user credentials.

> If the `valueFrom.secretKeyRef.namespace` is not specified, Notification Manager will get the secret in the [`defaultSecretNamespace`](./notification-manager.md#DefaultSecretNamespace).
> If the `defaultSecretNamespace` is not set, Notification Manager will get the secret in the namespace where the notification manager webhook is located.
