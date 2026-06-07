# Docker Isolated Network Design

## Goal

Docker プロバイダでセルを作成するとき、複製されたコンテナ群を必ずセル専用の独立ネットワークに所属させる。

削除時は、そのセル専用ネットワークも合わせて削除する。

## Scope

対象は Docker プロバイダで作成されるセル全体で、`volumeMode: copy` や DB copy のような個別機能には限定しない。

この変更では Docker アダプタがネットワーク名を自前で決定し、元コンテナの所属ネットワークは複製先の作成には使わない。

## Current Problem

現在の `internal/adapter/container/docker.go` は `docker inspect` 結果から元コンテナのネットワークを拾い、そのまま `docker run --network ...` に渡している。

このため、複製先セルのコンテナが元の compose ネットワークや共有ネットワークへ接続され、セル単位で隔離されない。

## Decision

今回の実装では、Docker アダプタ側でセル専用ネットワーク名を計算する。

計算規則は既存のセル命名と合わせて以下とする。

```text
paracell-<project>-<cell name>
```

これは現時点の `domain.CellFactory` が `cell.Containers.Network` に入れている値と同じだが、今回の方針では Docker アダプタがこの命名規則を直接持つ。

## Behavior

### Create

`CreateContainers` はコンテナ作成前に一度だけ以下を実行する。

```text
docker network create <cell network>
```

その後、各サービスの `docker run` では必ずそのセル専用ネットワークを指定する。

```text
docker run ... --network <cell network> ...
```

`docker inspect` は引き続き以下の情報の復元にだけ使う。

- image
- env
- mounts

inspect から得たネットワーク情報は使わない。

### Clean

`CleanContainers` は既存どおり各コンテナを `docker rm -f` で削除した後、最後にセル専用ネットワークを削除する。

```text
docker network rm <cell network>
```

コンテナ削除と同様に、ネットワーク削除はベストエフォートでよい。

## Architecture

Docker アダプタにセル専用ネットワーク名を返す小さな helper を追加する。

期待する分割は以下。

- セル専用ネットワーク名を計算する helper
- 必要ならネットワーク作成を行う helper
- 既存のコンテナ生成フロー
- 既存のクリーンアップフローにネットワーク削除を追加

ドメインモデルの `cell.Containers.Network` は残るが、今回の変更では Docker アダプタの実行判断には使わない。

## Error Handling

作成時:

- `docker network create` が失敗したら、その時点で `CreateContainers` を失敗させる
- 後続のコンテナ作成は行わない

削除時:

- コンテナ削除は現行同様ベストエフォート
- ネットワーク削除も同じくベストエフォート

## Testing

追加・更新するテスト観点:

- `CreateContainers` が最初に `docker network create <cell network>` を呼ぶ
- 各 `docker run` が inspect 由来ではなくセル専用ネットワーク名を使う
- `CleanContainers` が全コンテナ削除後に `docker network rm <cell network>` を呼ぶ
- volume copy や DB schema copy でもセル専用ネットワークが使われる

テストは既存の fake runner を使った unit test の範囲に留める。

## Non-Goals

- Docker ネットワーク名の設定をユーザー設定化すること
- 元ネットワークのエイリアスや追加ネットワークを複製すること
- Docker 以外の container provider へ同じ仕様を広げること
