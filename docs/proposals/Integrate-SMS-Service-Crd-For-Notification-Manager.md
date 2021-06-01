Currently, there is no configuration for SMS service platform among the numerous configurations of Notification-Manager. This proposal expects to design these crds to supplement it.

### **Introduction**
We have integrated many message tunnel components, such as DingTalk, Email, Slack, Wechat, Webhook. However, SMS service is very different from these previous channel configurations, and there are many SMS service providers.

To overcome these challenges, the integration process can be roughly divided into three steps.

Step 1, create a default config crd for different SMS providers.

Step 2, create a global receiever config for different SMS providers.

Step 3, trigger the alerts with notification deployments.

### **Design SmsConfig Crd**
A reference configuration is described as followings:

```
apiVersion: notification.kubesphere.io/v2beta1
kind: Config
metadata:
  labels:
    app: notification-manager
    type: default
  name: default-sms-config
spec:
  sms:
       defaultProvider: "aliyun"
       providers:
         aliyun: 
            signName: xxxx 
            templateCode: xxx
            accessKeyId: xxx
            accessKeySecret: xxx
         tencent:
            templateID: xxx
            smsSdkAppid: xxx
            sign:xxxx
```
            
### **Design SmsReceiever Crd**
A reference configuration is described as followings:

apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: global
  name: global-sms-receiver
spec:
  sms:
    enabled: true
    smsConfigSelector:
      matchLabels:
        type: tenant
        user: user1
    alertSelector:
      matchLabels:
        alerttype: auditing
    phoneNumbers:
    - 13612344321
    - 13812344321
    
### **How to trigger the alerts**
The different SMS service providers provide different method to send message. Therefore. the websocket server will be abstracted into common interfaces.

The whole process can be described as followings:

#### **Reconciliation**
The notifacation-manager operator is responsible for reconciliation while the crds creating or updating or deleting.

#### **Get alerts**
Secondly, the notification deployment will load the configuration of the sms receiever crd if it is enabled, using getMessage method by the defination of the alertSelector to filter alerts.

#### **Send notifications**
If the alerts are not empty, the sendMessage method can send notifications to the desired phoneNumbers with the selected SMS providers by using the common interfaces. In this way, the post request use the parameters of the selected SMS providers.
