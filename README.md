# Omni Orchestrator 
Omni-Orchestrator是统一构建平台(OBP)负责任务生命周期管理的组件，项目分为两部分:
1. Omni-Orchestrator: 框架负责跟任务的通用逻辑，比如任务创建，调度，管理，日志收集等，项目的目的在于集成现有通用任务调度框架集群管理及高并发能力，同时解耦底层的任务引擎，比如kubernetes，cadence, airflow，compass-ci等组件，在OBP整个系统中Omni-Orchestrator需要和特定的Worker一起配合才能完成某一项特定的任务执行逻辑。
2. Omni-Imager/Omni-Packer: 提供具体的构建能力，是真正执行底层逻辑的组件，比如Omni-Imager提供具体的构建，裁剪ISO的能力。

整个服务包含的组件如下:

![Architecture](./docs/assets/architecture.jpg)
# 目标
围绕任务管理，大致上需要满足以下几个诉求:
1. 任务的生命周期管理(创建/删除/查询)
2. 任务日志的查询(实时/归档)
3. 任务的事件推送(Notify on Job Changed)
4. 服务支持水平扩展
5. 支持多任务架构(X86&Aarch64)
6. 支持任务引擎多实例配置
7. 基础的API权限管理(Basic Auth?)
8. 具体的Worker(定制裁剪ISO&RPM&SRPM等)

# 核心点
## 服务框架
这里优先考虑Golang+Gin的配置，考虑点是能力模型+高性能。
## 数据库
数据库在Omni-Orchestrator里面核心的功能是任务状态存储+任务日志存储，结合实际的任务及日志存储量，有几点需要满足:
1. 数据量大，支持水平扩展，分布式。
2. 写大于读，更新少，数据能自动清理。
3. 数据结构简单。

最终我们选择了基于Cassandra设计数据模型，核心的PrimaryKey设计如下, 需要注意的是我们引入了`Service`，`Task`，及`Domain`三个基础字段方便后续不同构建场景，不同构建任务，以及不同租户的支持。
```shell
# Job
PRIMARY KEY  ((service, task), domain, job_date, job_id)
# Logs
PRIMARY KEY  ((service, task), domain, job_id, step_id, log_time)
) WITH CLUSTERING ORDER BY (domain ASC, job_id ASC, step_id ASC, log_time ASC);
```
## 任务引擎
基于kubernetes的任务平台构建方案是一开始的优选，而且CRD+Operator的模型或者现有云原生任务框架(Tekton)等都能比较容易的满足，而新引入一层主要是考虑到
在某些场景的支持上可能存在短板(比如任务并发，比如日志管理，比如除去X86及Aarch64的多架构支持)，因此整个服务前期主要引入Kubernetes作为任务引擎，同时支持配置其他同类型产品做补足。

## 任务扩展
整个构建平台涉及多种具体任务的开发，如ISO构建，RPM构建，SRPM打包，而每种任务的规格，参数，以及Worker版本都需要灵活配置，为满足快速开发和更新的需求，
任务的通用部分放在了核心的逻辑里面，具体的任务可通过插件+模板的方式快速开发，服务本身也支持SIGHUP重载模板文件，确保更新的有效及服务不中断，以ISO构建举例, 任务需要的资源都通过go template的模式单独配置:
```shell
➜  kubernetes_templates: tree -L 2
.
└── buildimagefromrelease
    ├── configmap.yaml
    └── job.yaml
➜  kubernetes_templates: cat buildimagefromrelease/configmap.yaml
apiVersion: v1
data:
  conf.yaml: |-
    working_dir: /data/omni-workspace
    debug: True
    user_name: root
    user_passwd: openEuler
    installer_configs: /etc/omni-imager/installer_assets/calamares-configs
    systemd_configs: /etc/omni-imager/installer_assets/systemd-configs
    init_script: /etc/omni-imager/init
    installer_script: /etc/omni-imager/runinstaller
    repo_file: /etc/omni-imager/repos/{{ index . "version" }}.repo
    use_cached_rootfs: True
    cached_rootfs_gz: /data/rootfs_cache/rootfs.tar.gz
  openEuler-customized.json: |-
    {{ index . "packages" }}
kind: ConfigMap
metadata:
  name: {{ index . "name" }}
  namespace: {{ index . "namespace" }}
```

## 水平扩展