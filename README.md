# DevSpeak AI

> Plataforma com IA para ajudar desenvolvedores a se prepararem para entrevistas técnicas internacionais em inglês.

![CI](https://github.com/Dudainfinity/devspeak-ai/actions/workflows/ci.yml/badge.svg)
![Deploy](https://github.com/Dudainfinity/devspeak-ai/actions/workflows/deploy.yml/badge.svg)
![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)
![AWS](https://img.shields.io/badge/AWS-EC2-FF9900?logo=amazonaws&logoColor=white)
![Terraform](https://img.shields.io/badge/Terraform-IaC-7B42BC?logo=terraform&logoColor=white)
![Kubernetes](https://img.shields.io/badge/Kubernetes-326CE5?logo=kubernetes&logoColor=white)

---

## Sobre o projeto

O **DevSpeak AI** combina dois objetivos:

1. **Produto** — uma plataforma de simulação de entrevistas técnicas em inglês com correção assistida por IA, voltada para programadores que buscam vagas internacionais.
2. **Engenharia** — uma arquitetura DevOps de referência, construída com práticas que empresas usam em produção: microsserviços em Go, containers, infraestrutura como código, orquestração com Kubernetes, pipelines CI/CD e observabilidade.

O repositório evolui em fases: cada serviço, peça de infra e automação é adicionado de forma incremental, refletindo um fluxo real de engenharia cloud-native.

---

## Arquitetura

```mermaid
graph LR
    Dev([Desenvolvedor]) -->|git push| GH[GitHub]
    GH --> CI[CI Pipeline]
    GH --> CD[Deploy Pipeline]
    CD -->|SSH| EC2
    subgraph AWS
        EC2[EC2 t3.micro] --> Nginx[Nginx<br/>:443 HTTPS]
        Nginx --> Container[speech-service<br/>127.0.0.1:8080]
    end
    User([Usuário final]) -->|HTTPS| Nginx
    TF[Terraform] -.->|provisiona| EC2
```

**URL pública:** https://13-220-172-249.sslip.io/health

Infraestrutura provisionada via **Terraform** (EC2 + Security Group + Key Pair).
Manifests **Kubernetes** existem para execução local em Minikube; migração para EKS está no roadmap.

> Diagramas completos (visão de sistema, topologia de deploy, sequência do CI/CD e decisões de arquitetura) em [`docs/architecture.md`](docs/architecture.md).

---

## Stack tecnológica

| Camada            | Tecnologia                                    |
|-------------------|-----------------------------------------------|
| Linguagem         | Go 1.24                                       |
| Containerização   | Docker, Docker Compose                        |
| Orquestração      | Kubernetes (Minikube — local)                 |
| Cloud             | AWS EC2 (`t3.micro`, Amazon Linux)            |
| IaC               | Terraform                                     |
| Reverse proxy     | Nginx + Let's Encrypt (HTTPS via sslip.io)    |
| CI/CD             | GitHub Actions                                |
| Observabilidade   | Prometheus, Grafana (local)                   |
| Versionamento     | Git + GitHub                                  |

---

## Estrutura do repositório

```
devspeak-ai/
├── speech-service/          # microsserviço Go (endpoint /health, /metrics)
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── api/                     # backend principal (planejado)
├── frontend/                # interface do usuário (planejado)
├── infra/
│   ├── main.tf              # Terraform: EC2 + Security Group
│   ├── terraform/           # módulos auxiliares
│   ├── nginx/
│   │   └── devspeak.conf    # config Nginx (HTTPS + reverse proxy)
│   ├── scripts/
│   │   └── setup-nginx-https.sh   # bootstrap idempotente do TLS
│   └── k8s/                 # manifests Kubernetes
│       ├── speech-deployment.yaml
│       └── speech-service.yaml
├── docs/                    # documentação adicional
├── .github/workflows/
│   ├── ci.yml               # build + lint a cada push
│   └── deploy.yml           # deploy automático para EC2 via SSH
├── docker-compose.yml       # ambiente local (speech-service + postgres)
└── README.md
```

---

## Rodando localmente

Pré-requisitos: Docker e Docker Compose v2.

```bash
git clone https://github.com/Dudainfinity/devspeak-ai.git
cd devspeak-ai
docker compose up -d --build
```

Verificação:

```bash
curl http://localhost:8080/health
# DevSpeak AI Speech Service Running
```

### Apenas o serviço Go (sem compose)

```bash
cd speech-service
docker build -t speech-service .
docker run -d -p 8080:8080 --name speech-service speech-service
```

---

## Deploy automatizado (CI/CD)

A cada `git push` na branch `main`, dois workflows são disparados em paralelo:

| Workflow      | Arquivo                       | O que faz                                          |
|---------------|-------------------------------|----------------------------------------------------|
| **CI**        | `.github/workflows/ci.yml`    | `go mod tidy`, `go build`, build da imagem Docker  |
| **Deploy**    | `.github/workflows/deploy.yml`| SSH na EC2, `git pull`, rebuild e restart do container |

### Secrets necessários

Configurados em **Settings → Secrets and variables → Actions** do repositório:

| Secret      | Conteúdo                                                                 |
|-------------|--------------------------------------------------------------------------|
| `EC2_HOST`  | IP público da instância EC2                                              |
| `EC2_USER`  | usuário SSH (`ec2-user` para Amazon Linux)                               |
| `EC2_KEY`   | conteúdo completo da chave privada `.pem` (incluindo `BEGIN/END`)        |

> Para evitar problemas de quebra de linha ao colar a chave no navegador, use a CLI: `gh secret set EC2_KEY < ~/.ssh/devspeak-key.pem`

### Setup inicial da EC2 (uma única vez)

```bash
ssh -i ~/.ssh/devspeak-key.pem ec2-user@<EC2_HOST>
sudo yum install -y git docker
sudo systemctl enable --now docker
sudo usermod -aG docker $USER   # reabra a sessão SSH
cd ~ && git clone https://github.com/Dudainfinity/devspeak-ai.git
```

A partir daí, todo `git push` na `main` atualiza a aplicação online automaticamente.

### HTTPS com Nginx e Let's Encrypt

A aplicação é servida em `https://13-220-172-249.sslip.io` via Nginx como reverse proxy, com certificado Let's Encrypt (renovação automática diária via cron).

Setup inicial (uma única vez na EC2):

```bash
cd ~/devspeak-ai
git pull origin main
bash infra/scripts/setup-nginx-https.sh
```

O script é **idempotente** — pode ser re-executado sem efeito colateral. Ele:

1. Instala `nginx` e `certbot`
2. Sobe nginx com config HTTP mínima para validar o desafio ACME
3. Solicita o certificado ao Let's Encrypt via webroot
4. Substitui pela config completa (HTTPS + reverse proxy + headers de segurança)
5. Agenda renovação automática (`certbot renew` diário às 03:00 com reload do nginx)

> **sslip.io** é um serviço DNS gratuito que mapeia hostnames para IPs codificados no próprio nome (`13-220-172-249.sslip.io` → `13.220.172.249`). Permite obter HTTPS real do Let's Encrypt sem registrar um domínio. Quando houver um domínio próprio, basta substituir o `server_name` no `infra/nginx/devspeak.conf`.

---

## Provisionando a infraestrutura

```bash
cd infra
terraform init
terraform plan
terraform apply
```

Recursos criados:

- Instância EC2 `t3.micro`
- Security Group com porta `22` (SSH) e `8080` (aplicação)
- Output com o IP público da máquina

---

## Observabilidade

O serviço Go expõe métricas no padrão Prometheus:

```bash
curl http://<EC2_HOST>:8080/metrics
```

Dashboards Grafana foram configurados em ambiente local conectando a um Prometheus que coleta métricas do container. A pilha completa em produção está no roadmap.

---

## Roadmap

**Infra & DevOps**
- [ ] Nginx como reverse proxy na EC2
- [ ] Domínio próprio + certificado HTTPS (Let's Encrypt)
- [ ] Publicação de imagens no GHCR ou Docker Hub
- [ ] Ambientes separados (staging / production)
- [ ] Rollback automatizado no pipeline
- [ ] Migração de Kubernetes local para EKS
- [ ] Monitoramento contínuo da EC2 (CloudWatch + alertas)

**Produto**
- [ ] Frontend inicial (web)
- [ ] API principal com autenticação de usuários
- [ ] Integração com OpenAI para geração e correção de respostas
- [ ] Transcrição de áudio (speech-to-text)
- [ ] Simulador completo de entrevistas técnicas
- [ ] Dashboard do candidato com histórico e métricas de evolução

---

## Licença

A definir.
