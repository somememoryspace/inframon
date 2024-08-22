# Inframon
Make Infrastructure Monitoring Easy Again. Simple easy monitoring service with no frills. Monitor services and just see exactly what you need to worry about. 

Consider the motto: _If the HealthCheck is Passing, Nothing to Worry About. Carry-On._

- **Platforms**: Kubernetes | LXC | Docker | Podman | Virtual Machine | Bare Metal |
- **Supported** **Architectures**: Linux ARM64 AMD64 | macOS ARM64

## Current Development Builds
[![Build Dev](https://github.com/somememoryspace/inframon/actions/workflows/build-dev-multi.yml/badge.svg)](https://github.com/somememoryspace/inframon/actions/workflows/build-dev-multi.yml)

## Current Releases v1.0.1
[![Build Release](https://github.com/somememoryspace/inframon/actions/workflows/build-release-multi.yml/badge.svg)](https://github.com/somememoryspace/inframon/actions/workflows/build-release-multi.yml)


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

## How It Works
- Services are loaded into a configuration file and loaded on Inframon start-up. 
- Based on the set timeout intervals, the services are checked via the set protocol. 
- Services reported as disrupted are reported via the notification channels configured. 
- If enabled, HealthCheck reports provide a scheduled report of continually failing services, or an all OK status. 

## Notification Examples
### Discord Webhook
<p align="left">
  <img src="assets/discord.png" alt="Discord Webhook Example">
</p>

### SMTP Email
<p align="left">
  <img src="assets/smtp.png" alt="Discord Webhook Example">
</p>

## Security Posture
Inframon considers security in the implementation model and has certain capabilities in place:
- Stateless-Type Architecture:
  - No Database to Configure, Manage, Migrate, Maintain
  - No File Store for Application State
  - Single Configuration File -> Initialized on Runtime -> Instance Runs
- Routine Code and Security Scanning using GoSec, StaticCheck, Gitleaks, and Trivy.
- Root or Rootless Operation Mode
- Toggle Mode for TLS Verification (Common For This Type of Tool)
- Contaier Images use an Alpine slim base image with minimal added package dependencies.
- Container Images are built to run with a non-root user within the image.

### Disclosure Request
For any security issues found, please open a Github Issue for review and provide detailed observations. 

### GoSec Security Results
| GoSec Issue ID | Severity | Response Statement |
|----------|----------|---------------------|
| G402 | HIGH | TLS verification skip is configurable and used only when necessary for internal services that do not used an appropriate TLS certificate. |
| G304 | MEDIUM | File paths are from trusted config and command line arguments when the service is started, not with regular user input. |

### Trivy Container Security Results
| Trivy Issue ID | Severity | Response Statement |
|----------|----------|---------------------|
| N/A | N/A | No Security Issues Found. |

## Define Configuration File
Define a configuration file to load in ICMP or HTTP based monitors. Additionally, define instance specific configuration. 
```yaml
icmp:
  - address: "10.91.255.214"
    service: "SomeMachine"
    timeout: 5
    failureTimeout: 10
    retryBuffer: 5
    networkZone: "DMZ"
    instanceType: "VirtualMachine"

http:
  - address: "https://loadbalancer.domain.net"
    service: "service-loadbalancer"
    timeout: 60
    failureTimeout: 10
    skipVerify: true
    retryBuffer: 5
    networkZone: "GATEWAYS"
    instanceType: "LXC"

configuration:
    stdOut: true
    healthCheckTimeout: 5
    discordWebhookDisable: false
    healthCron: "0 */12 * * *"
    healthCronDisable: true    
    healthCronWebhookDisable: false
    healthCronSmtpDisable: false
    discordWebhookUrl: "https://discord.com/api/webhooks/***********************************"
    smtpDisable: false
    logFileSize: "10MB"
    maxLogFileKeep: 5
    smtpHost: "smtp.sendgrid.net"
    smtpPort: "587"
    smtpFrom: "donotreply@domain.net"
    smtpUsername: "USERNAME"
    smtpPassword: "PASSWORD"
    smtpTo: "email@domain.net"

```

## Docker Deployment

### Pull the Container Image
You can pull the pre-built container image from the GitHub Container Registry:

#### Latest Image
```bash
$ docker pull ghcr.io/somememoryspace/inframon:latest
```

#### Development Image
```bash
$ docker pull ghcr.io/somememoryspace/inframon:dev
```

### Create a Docker Directory and an Empty Compose File
```bash
$ mkdir inframon
$ cd inframon
$ touch docker-compose.yaml
```

### Example Compose File
```yaml
version: '3.8'
services:
  inframon:
    container_name: inframon
    image: ghcr.io/somememoryspace/inframon:latest
    environment:
      - CONFIG_PATH=/config/config.yaml
    volumes:
      - ./config:/config
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
```yaml
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
```yaml
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
        image: ghcr.io/somememoryspace/inframon:latest
        imagePullPolicy: Always
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
```bash
$ mv ./inframon /usr/local/bin/inframon
$ chmod +x /usr/local/bin/inframon
```

### Example systemD file running on LXC:
```ini
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
This project is licensed under the Apache 2.0 License - see the LICENSE file for details.