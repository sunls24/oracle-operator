### Run

需指定 CLI 的镜像地址，指定 NS 命名空间可选

```shell
make run CLI=10.10.236.107:8099/oracle/instantclient:19-gotty-3 NS=default EXPORTER=10.10.236.107:8099/k8s/oracle/oracledb_exporter:custom
```

### Build

```shell
export IMG="10.10.236.107:8099/k8s/oracle/oracle-operator:0.1"
docker build -t $IMG .
docker push 10.10.236.107:8099/k8s/oracle/oracle-operator:0.1
```

### Deploy

```shell
kubectl create -f config/samples
```

### QA

#### gcr.io/distroless/static:nonroot 镜像无法 pull

需要设置代理，让 `docker pull` 走代理网络  
通过设置 `systemd` 的代理测试可行：https://docs.docker.com/config/daemon/systemd/