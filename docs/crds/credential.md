# Credential

## Overview

The `credential` used to store user credentials, like password, token, app secret, etc.

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
- `valueFrom.secretKeyRef` - The secret that used to store user credentials.
- `valueFrom.secretKeyRef.name` - The name of secret that used to store user credentials.
- `valueFrom.secretKeyRef.namespace` - The namespace of secret that used to store user credentials.
- `valueFrom.secretKeyRef.key` - The key of secret that used to store user credentials.

> If the `valueFrom.secretKeyRef.namespace` does not specify, notification manager will get the secret in the [`defaultSecretNamespace`](./notification-manager.md#DefaultSecretNamespace).
> If the `defaultSecretNamespace` is not set, notification manager will get the secret in the namespace where the notification manager webhook is located.
