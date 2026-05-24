# pdev 設計

## 概要

`pdev` は、個人開発者が複数の開発環境を並列に扱うための Go 製 CLI である。1つの開発環境を `cell` と呼ぶ。cell は、現在のリポジトリから作られた作業用 source、コピーされた Docker コンテナ群、作業用の detached session で構成される。

MVP では Docker Compose を使って cell のコンテナを作らない。代わりに、template で明示された起動中コンテナを `docker inspect` し、その情報をもとに Docker CLI で cell 専用コンテナを起動する。

## 目的

- issue 識別子と template から並列開発用 cell を作成する。
- CLI 名は `pdev` にする。
- `git` や `tmux` などの実装詳細を、ユーザー向けの主語にしない。
- 指定された起動中コンテナを cell 専用コンテナとしてコピーする。
- source path、Docker network、container name、volume、port、session name を cell ごとに分離する。
- 初期設定、cell の作成、削除を CLI で扱えるようにする。
- MVP の状態はプロジェクト内に保存し、将来 `~/.pdev` へ移せる境界を残す。

## 対象外

- チーム向けの調整機能。
- GitHub issue 連携。
- TUI または Web UI。
- Docker Compose による cell 作成。
- Kubernetes、devcontainer、リモートコンテナ管理。
- 起動中プロセス状態そのもののコピー。
- Docker secrets、configs、複雑な multi-network、細かい Docker edge case の完全対応。

## 概念

### Template

template は cell を作るための設計図である。以下を持つ。

- `repository`: 現在のリポジトリから cell 用 source を作る方法。
- `containers`: コピーする起動中コンテナと、その service role。
- `session`: cell で開く作業用 terminal window。

### Cell

cell は独立した開発環境である。以下を持つ。

- 内部用の安定した `id`。UUID または ULID を想定する。
- ユーザーが指定する `issue`。例: `123`。
- 表示や命名に使う `name`。MVP では issue から生成する。
- `.pdev/cells/<name>/source` 配下の source path。
- 専用 Docker network。
- service role ごとのコンテナ。
- detached session。
- `.pdev/state.json` に保存される状態。

### Containers

コンテナは service role で設定する。たとえば `web` や `db` が、起動中コンテナ `myapp-web` や `myapp-db` を参照する。

`pdev` は `docker inspect` でコンテナ設定を読み取り、`docker run` で cell 専用コンテナを起動する。

## 設定

プロジェクトルートに `.pdev.yml` を置く。

```yaml
project:
  name: myapp

templates:
  webapp:
    repository:
      branchPrefix: feat/
      base: main

    containers:
      services:
        web:
          sourceContainer: myapp-web
        db:
          sourceContainer: myapp-db

    session:
      windows:
        - name: editor
          command: nvim .
        - name: web-log
          command: docker logs -f ${containers.web.name}
        - name: db
          command: docker exec -it ${containers.db.name} psql
```

MVP では現在のリポジトリのみを対象にする。将来、template に repository path や remote を追加できる余地を残す。

## CLI

```bash
pdev init
pdev create <issue> --template <template>
pdev remove <cell>
pdev remove <cell> --force
```

MVP のユーザー向けコマンドは `init`、`create`、`remove` に絞る。

`pdev init` は空の初期設定を作る。検出は行わず、`.pdev.yml` と `.pdev/` を作成する。`.pdev.yml` が既にある場合は失敗する。

`pdev create` は cell の作成、コンテナ起動、detached session 作成まで行うが、session には入らない。session に入る操作は MVP の `pdev` コマンドとしては提供せず、ユーザーが通常の session tool で入る。

MVP では `<cell>` は `name` または `issue` で解決できる。内部処理では安定した `id` を使う。

## アーキテクチャ

実装は Go の単一バイナリ CLI とする。DDD と Clean Architecture を使い、domain が中心になるように依存方向を揃える。外部ツールは adapter 経由で呼び出す。

```text
cmd/pdev
  CLI entrypoint。引数解析だけを行う。

internal/app
  dependency wiring。

internal/usecase
  InitProjectUseCase
  CreateCellUseCase
  RemoveCellUseCase

  Port Interface:
    InitConfigPort
    ConfigPort
    CellStatePort
    SourcePort
    ContainerPort
    SessionPort
    IDGenerator
    Clock

internal/domain
  Domain Model:
    Cell
    Template
    Issue
    CellID
    CellName
    TemplateName
    Source
    ContainerService
    Session

  Domain Service:
    CellFactory
    CellNameGenerator
    CellUniquenessChecker

internal/adapter
  config:
    YAMLConfigAdapter
  state:
    JSONCellStateAdapter
  source:
    GitSourceAdapter
  container:
    DockerCLIAdapter
  session:
    TmuxAdapter
  system:
    OSCommandRunner
```

依存方向:

```text
cmd -> app -> usecase -> domain
adapter -> usecase ports
app -> adapter
```

usecase は port interface を定義し、それらを使って手順を組み立てる。adapter は usecase port を実装し、Docker、tmux、filesystem、repository 操作などの外部詳細を閉じ込める。

Domain Service は外部 I/O を行わない。たとえば `CellUniquenessChecker` は既存 cell 一覧を引数で受け取り、重複判定だけを行う。既存 state を読む処理は usecase port の `CellStatePort` 実装が担当する。

Entity の値は通常、entity のメソッドで変更する。子 entity を変更する場合は、aggregate root である `Cell` のメソッドから子 entity のメソッドを呼び出し、整合性を保つ。Go のフィールドは公開してよいが、domain 内の変更経路はメソッドとして表現し、テストで固定する。

方針は「状態モデルは明示的に持つが、実装は薄く保つ」。init、create、remove を確実に行うために必要な状態は持つが、重い orchestration system にはしない。

## 初期化フロー

`pdev init` は以下を行う。

1. `.pdev.yml` が存在しないことを確認する。
2. `.pdev/` を作る。
3. 空の `.pdev.yml` を作る。

生成する `.pdev.yml`:

```yaml
project:
    name: ""
templates:
    default:
        repository:
            branchPrefix: feat/
            base: main
        containers:
            services: {}
        session:
            windows: []
```

MVP では git、Docker Compose、起動中コンテナの検出は行わない。

## 作成フロー

`pdev create 123 --template webapp` は以下を行う。

1. `.pdev.yml` を読む。
2. template を解決する。
3. `.pdev/state.json` を読む。
4. 同じ issue または name の cell がないことを確認する。
5. cell 用の安定 ID を生成する。
6. 現在のリポジトリから cell 用 source を作る。
7. 設定された source container をそれぞれ inspect する。
8. cell 専用 Docker network を作る。
9. named volume を cell 専用 volume にコピーする。
10. repository を指す bind mount を cell source path に向け直す。
11. exposed port / published port に空き host port を割り当てる。
12. `docker run` で cell container を起動する。
13. detached session を作成する。
14. cell record を `.pdev/state.json` に保存する。

## コンテナコピー

service role ごとに、`pdev` は以下をコピーする。

- image reference。
- environment variables。
- command と entrypoint。
- working directory。
- exposed port と published port の意図。
- named volume。cell 専用 volume にコピーする。
- bind mount。repository mount は cell source path に向け直す。

`pdev` は以下を cell 用に新しく作る。

- Docker network。
- container name。
- cell 専用 volume。
- host port。

MVP では起動中プロセス状態そのものはコピーしない。bind mount または volume 上にない container layer 内のファイルは、image に含まれていない限り引き継がない。

## Session

template は session windows を定義する。MVP では window 単位のみを扱い、pane layout は後で追加する。

`pdev create` では session を detached で作る。MVP では session に入るための `pdev` コマンドは提供しない。

session command では以下の変数を使える。

- `${cell.id}`
- `${cell.issue}`
- `${cell.name}`
- `${cell.source}`
- `${project.name}`

## State

MVP の状態は `.pdev/state.json` に保存する。

```json
{
  "cells": [
    {
      "id": "01HX8K7R4V9S4M8X8D9YQ2J3CA",
      "issue": "123",
      "name": "123",
      "template": "webapp",
      "branch": "feat/123",
      "sourcePath": ".pdev/cells/123/source",
      "containers": {
        "network": "pdev-123",
        "services": {
          "web": {
            "containerName": "pdev-123-web",
            "sourceContainer": "myapp-web",
            "ports": {
              "3000/tcp": 31842
            }
          },
          "db": {
            "containerName": "pdev-123-db",
            "sourceContainer": "myapp-db",
            "volumes": [
              "pdev-123-db-data"
            ]
          }
        }
      },
      "session": {
        "name": "pdev-myapp-123"
      },
      "createdAt": "2026-05-20T00:00:00Z"
    }
  ]
}
```

create / remove の内部検証では、state だけでなく実体も確認する。

- source path が存在するか。
- container が存在するか、running / stopped のどちらか。
- network が存在するか。
- session が存在するか。
- state の port と Docker 上の port が一致しているか。

## エラー処理

以下の場合、作成は失敗する。

- `.pdev.yml` がない。
- template がない。
- 同じ issue または name の cell が既にある。
- source container が存在しない。
- source container が停止している。
- source 作成に失敗した。
- volume copy に失敗した。
- port 割り当てに失敗した。
- container 起動に失敗した。
- session 作成に失敗した。

失敗時、`pdev` は作成済みの source path、network、container、volume、session を可能な範囲で削除する。掃除しきれない場合は partial cell として state に残し、`pdev remove <cell> --force` で掃除できるようにする。

## テスト

単体テスト:

- config parsing。
- template 解決。
- ID、issue、name 生成。
- state store の読み書き。
- cell reference 解決。
- `docker inspect` から `docker run` 設定への変換。
- port allocation。
- session command の変数展開。

結合テスト:

- nginx のような単純な起動中コンテナをコピーできる。
- named volume を持つコンテナをコピーできる。
- cell の create が動く。
- remove で source、network、container、volume、session が掃除される。
- 意図的な失敗後に partial state または cleanup が働く。

## 今後

- 同じ state model の上に TUI を追加する。
- project config は `.pdev.yml` に残し、実行状態は `~/.pdev` へ移す。
- suffix によって同じ issue の複数 cell を許可する。
- GitHub issue title lookup。
- template に repository path / remote を追加する。
- session の pane layout。
- seed container または reusable snapshot。
- Docker 機能の対応範囲拡張。
