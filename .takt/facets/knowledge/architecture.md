# アーキテクチャ知識

このリポジトリは Go の CLI アプリケーションとして、`domain` / `usecase` / `adapter` / `app` の境界を持つ。実装時は、処理をどこに置くかを先に判断してから変更する。

## 全体構造

| 層 | 主な責務 | 置かないもの |
|----|----------|--------------|
| `internal/domain` | モデル、不変条件、ドメイン生成、状態変更 | 外部コマンド、ファイル I/O、設定ファイル形式 |
| `internal/usecase` | ユースケースの手順、port 定義、domain と adapter の接続 | Docker/tmux/git のコマンド詳細、CLI 解析 |
| `internal/adapter` | 外部システム、永続化、設定、表示、コマンド実行 | 業務判断、ユースケースの順序決定 |
| `internal/app` | CLI コマンド、依存の組み立て、標準入出力 | domain rule、adapter の詳細実装 |

依存方向は `app -> usecase -> domain` と `adapter -> usecase/domain` を基本にする。`domain` は他層を知らない。`usecase` は port 越しに adapter を使う。

## domain

domain は「paracell が扱う概念」を表す。

扱うもの:

- `Cell`、`Source`、`Container`、`Session`、`Template` などの状態
- 必須値の検証
- 状態変更に伴う不変条件
- domain model の生成 factory
- 外部形式に依存しない値の導出

避けるもの:

- YAML/JSON の field 名に依存した処理
- Docker/tmux/git の用語やコマンド
- ファイルシステムや process 実行
- CLI flag の解釈
- config の優先順位解決

判断基準:

- その処理は外部ツールが変わっても残るか。残るなら domain の候補
- その処理は外部形式や実行環境を知らないと書けないか。必要なら adapter/app/usecase 側

## usecase

usecase は「何をどの順序で実行するか」を表す。

扱うもの:

- config/state/provider を port 経由で取得する
- domain factory で cell を生成する
- source、container、session などの port を順に呼ぶ
- 途中失敗時の停止、削除、状態保存などの手順を決める
- usecase が必要とする interface を定義する

避けるもの:

- `docker run` や `tmux new-window` のような具体コマンド
- YAML/JSON parser の詳細
- adapter 実装型への依存
- CLI 引数や標準出力の整形

usecase のコードは、処理順序が読めることを優先する。1箇所の手順を無理に action table 化して、失敗箇所や責務が読みにくくなるなら明示的に書く。

## adapter

adapter は外部世界との変換を担当する。

扱うもの:

- Docker、tmux、git などのコマンド引数
- config file、state file、YAML/JSON の読み書き
- file copy、path 解決、worktree などの I/O
- TUI/view model など表示に近い変換
- 外部コマンドの stdout/stderr の parse

避けるもの:

- usecase の実行順序を adapter 内で決めること
- domain の不変条件を adapter 側だけで守ること
- 設定の優先順位を複数 adapter に散らすこと

外部コマンドは runner 経由で実行し、テストでは runner の引数や parser の結果を検証する。

## app

app は CLI と依存注入の層。

扱うもの:

- command/subcommand/flag の解析
- 作業ディレクトリ、config adapter、state adapter、provider factory の組み立て
- usecase input の生成
- 標準出力、標準エラー、終了コードに近い処理

避けるもの:

- domain model を直接細かく変更すること
- Docker/tmux/git のコマンド引数を組み立てること
- usecase を通さず adapter を直接 orchestration すること

## port と factory

usecase が必要とする操作を port として定義する。interface は実装側ではなく利用側の言葉で作る。

良い port:

- `CreateSource`
- `CreateContainers`
- `CreateSession`
- `RemoveSession`
- `EnterSession`

避ける port:

- 外部ツール名が入ったメソッド
- 内部手順を細かく分けすぎたメソッド
- 呼び出し側が自然に持っていない引数を要求するメソッド
- 実装の都合で巨大化した interface

provider factory は解決済み provider config を受け取り、usecase port を返す。factory 内では「対応 provider の選択」と「未対応 provider のエラー」を扱う。業務判断や fallback は置かない。

## 解決と正規化

設定、provider、パス、テンプレート変数、optional/default は、実行前に一度だけ解決する。

基準:

- raw config を深い層まで運ばない
- 同じ優先順位ロジックを複数箇所に置かない
- 表示用、実行用、保存用で別々に解決しない
- 解決済みの値を domain/usecase/adapter に渡す

例:

- config adapter が YAML を読み、domain/usecase が扱いやすい構造へ変換する
- usecase は `cfg.Providers` のような解決済み値を factory に渡す
- adapter は渡された値をそのまま外部コマンドへ変換する

## フェーズ分離

処理は次の順に分けて考える。

1. 入力を読む
2. 設定とテンプレートを解決する
3. domain model を生成・検証する
4. 外部副作用を実行する
5. 状態を保存する
6. 表示する

ループ内で raw input の解釈と副作用実行を混ぜない。反復処理が必要な場合も、先に解決済みの構造へ変換し、ループ内は実行に寄せる。

## 状態と永続化

domain の正規状態と、JSON/YAML などの保存形式を混同しない。

基準:

- domain は保存形式に引きずられない
- 保存形式の互換性は adapter の load/save test で守る
- 同じ事実を複数 field に持たない
- state 更新は usecase の成功/失敗の流れと整合させる

## テスト方針

層ごとに検証する対象を分ける。

| 層 | 検証対象 |
|----|----------|
| domain | 不変条件、生成、状態変更 |
| usecase | port 呼び出し順、エラー時の停止、state 更新 |
| adapter | コマンド引数、外部形式の parse、load/save |
| app | CLI parse、wiring、usecase input |

外部コマンドを直接叩くテストではなく、runner/fake port を使って境界を検証する。統合確認が必要な場合は、通常の unit test と分けて扱う。

## レビュー観点

- 変更が正しい層に置かれているか
- domain が外部詳細を知らないか
- usecase が orchestration として読めるか
- adapter が外部形式と副作用に閉じているか
- app が CLI と wiring 以上の責務を持っていないか
- interface が利用側の都合で設計されているか
- 設定、状態、永続化、README、サンプルが同じ契約を向いているか
- エラー時に途中状態が壊れないか
- テストが層の責務に合っているか
