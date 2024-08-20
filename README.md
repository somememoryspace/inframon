# Inframon: Stupid Easy Infrastructure Monitoring

## Current Releases
![Build Docker Image](https://github.com/somememoryspace/inframon/actions/workflows/build-docker-actions.yml/badge.svg)

![Build Go Binary](https://github.com/somememoryspace/inframon/actions/workflows/build-go-actions.yml/badge.svg)

## Ready to Use Features
- **ICMP Monitoring**: Ping servers and network devices to check their availability.
- **HTTP Monitoring**: Check the health of web services and APIs.
- **Flexible Configuration**: Setup ICMP and HTTP Monitors within the config.yaml file.
- **Notifications**: 
  - Discord Webhook Integration
  - SMTP Email Integration
- **Scheduled Health Checks**: Configurable cron-like scheduling for periodic status summaries.
- **Logging**: Detailed logging with rotation capabilities.
- **Privilege Mode**: Option to run with elevated privileges using --root_user set to true. Supports Docker, VM, LXC, Kubernetes. 

## Coming Soon
- **Routine Bugfixes**: Corrective bugfixes as they are discovered and reported.
- **Refinements**: Ongoing changes to output format for Discord Webhook and SMTP Notifications.
- **Secrets Management**: Migrate credentials in config file to a secure secrets management solution.
---

## Docker Deployment
```bash
$ git clone https://github.com/somememoryspace/inframon
```

### Build the Image
```bash
$ cd scripts
$ ./builddocker.sh
$ cd ../
```

### Create a Docker Directory and an Empty Compose File
```bash
$ mkdir inframon
$ cd inframon
$ touch docker-compose.yaml
```

### Example Compose File
```bash
version: '3.8'
services:
  inframon:
    container_name: inframon
    image: inframon:latest
    environment:
      - CONFIG_PATH=/config/config.yaml
    volumes:
      - ../config:/config
      - /etc/localtime:/etc/localtime:ro
      - /etc/timezone:/etc/timezone:ro 
    network_mode: bridge
```

### Run the Container
```bash
$ docker compose up -d
```

### Get Some Logs From the Inframon Container
```bash
$ docker logs --follow inframon

2024/08/19 00:40:33 utils.go:192: {"Type":"STARTUP","Message":"rootUserMode :: [false]","Event":"INFO"}
2024/08/19 00:40:33 utils.go:192: {"Type":"STARTUP","Message":"stdOut :: [true]","Event":"INFO"}
2024/08/19 00:40:33 utils.go:192: {"Type":"STARTUP","Message":"healthCheckTimeout :: [5]","Event":"INFO"}
2024/08/19 00:40:33 utils.go:192: {"Type":"STARTUP","Message":"discordWebhookDisable :: [false]","Event":"INFO"}
2024/08/19 00:40:33 utils.go:192: {"Type":"STARTUP","Message":"smtpDisable :: [false]","Event":"INFO"}
2024/08/19 00:40:33 utils.go:192: {"Type":"STARTUP","Message":"Starting Inframon","Event":"INFO"}
```


---


## Kubernetes Deployment

You can deploy Inframon on a Kubernetes cluster using the following steps:

### Create a Namespace
```bash
$ kubectl create namespace inframon
```

### Create a ConfigMap (Example)
```bash
apiVersion: v1
kind: ConfigMap
metadata:
  name: inframon-config
  namespace: inframon
data:
  config.yaml: |
    configuration:
      stdout: true
      healthCheckTimeout: 300
      discordWebHookDisable: false
      discordWebHookURL: "https://discord.com/api/webhooks/your-webhook-url"
      smtpDisable: true
    icmp:
      - address: "8.8.8.8"
        service: "Google DNS"
        retryBuffer: 3
        timeout: 5
        failureTimeout: 10
        networkZone: "Public"
        instanceType: "DNS"
    http:
      - address: "https://api.example.com"
        service: "Example API"
        retryBuffer: 3
        timeout: 5
        failureTimeout: 10
        skipVerify: false
        networkZone: "Public"
        instanceType: "API"
```

### Apply the ConfigMap
```bash
$ kubectl apply -f inframon-configmap.yaml
```

### Create a Deployment
```bash
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inframon
  namespace: inframon
spec:
  replicas: 1
  selector:
    matchLabels:
      app: inframon
  template:
    metadata:
      labels:
        app: inframon
    spec:
      containers:
      - name: inframon
        image: inframon:latest
        imagePullPolicy: IfNotPresent
        args: ["--config", "/config/config.yaml"]
        volumeMounts:
        - name: config
          mountPath: /config
      volumes:
      - name: config
        configMap:
          name: inframon-config
```

### Apply the Deployment
```bash
$ kubectl apply -f inframon-deployment.yaml
```

### Check the Running Pods
```bash
$ kubectl get pods -n inframon

NAME                       READY   STATUS    RESTARTS   AGE
inframon-7b9f8d6f5-x9zqm   1/1     Running   0          2m37s
```

### Get Pod Logs
```bash
$ kubectl logs -f deployment/inframon -n inframon

2024/08/19 00:49:53 utils.go:192: {"Type":"ICMP OK","Message":"Address: [192.168.60.4] Service: [frontdoor-camera] NetworkZone: [IPCAM] InstanceType: [IoT] Latency: [22.033ms]","Event":"INFO"}
2024/08/19 00:49:53 utils.go:192: {"Type":"ICMP OK","Message":"Address: [192.168.60.7] Service: [houseleft-camera] NetworkZone: [IPCAM] InstanceType: [IoT] Latency: [15.49925ms]","Event":"INFO"}
```

---


## Classic Deployment
### Get Repository
```bash
$ git clone https://github.com/somememoryspace/inframon
```

### Build the Binary
```bash
$ cd scripts
$ ./buildbinary.sh
$ cd ../
```

### Place Binary Into Appropriate Directory
```
$ mv ./inframon /usr/local/bin/inframon
$ chmod +x /usr/local/bin/inframon
```

### Example systemD file running on LXC:
```bash
[Unit]
Description=Inframon Service
After=network.target

[Service]
ExecStart=/usr/local/bin/inframon --config /root/inframon/config/config.yaml --root_user=True
WorkingDirectory=/root/inframon
User=root
Group=root
Restart=always

[Install]
WantedBy=multi-user.target
```

### Enable and Start the Service
```bash
$ systemctl daemon-reload
$ systemctl enable inframon
$ systemctl start inframon
```

### Check the Service
```bash
$ systemctl status inframon

● inframon.service - Inframon Service
     Loaded: loaded (/etc/systemd/system/inframon.service; enabled; preset: enabled)
     Active: active (running) since Mon 2024-08-19 04:17:53 UTC; 13min ago
   Main PID: 233 (inframon)
      Tasks: 7 (limit: 154364)
     Memory: 13.6M (peak: 16.5M swap: 3.7M swap peak: 7.2M)
        CPU: 1.509s
     CGroup: /system.slice/inframon.service
             └─233 /root/inframon/inframon --config /root/inframon/config/config.yaml --root_user=True

Aug 19 04:31:04 inframon-dev-lxc inframon[233]: 2024/08/19 04:31:04 utils.go:192: {"Type":"ICMP OK","Message":"Address: [10.10.100.20] Service: [dataplane-virts-one] NetworkZone: >
Aug 19 04:31:04 inframon-dev-lxc inframon[233]: 2024/08/19 04:31:04 utils.go:192: {"Type":"ICMP OK","Message":"Address: [10.10.100.109] Service: [guacamole] NetworkZone: [VIRTS] I>
```

### Usage
Run Inframon with the following command:

```bash
$ inframon --config /path/to/config.yaml --logpath /path/to/logs --logname inframon.log [--root_user]
```

## License
This project is licensed under the MIT License - see the LICENSE file for details.