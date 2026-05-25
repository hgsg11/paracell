# コーディングポリシー

速さより丁寧さ、実装の楽さよりコードの正確さを優先する。Go の標準的な書き方と、このリポジトリの既存設計を優先する。

このポリシーは「守るべきルール」と「判断例」を分けて読む。例は代表例であり、文字列や型名が一致しなくても同じ構造の問題なら同じ基準で判断する。

## 原則

| 原則 | ルール |
|------|------|
| Simple > Easy | 書く都合より、読み手が追える構造を優先する |
| DRY | 同じ理由で変わる重複を排除する |
| Fail Fast | 不正な入力、失敗、未解決値を早期に検出する |
| 小さな関数 | 1つの関数では同じ抽象度の処理だけを扱う |
| 明示的な契約 | 型、interface、設定、永続化、CLI、テストを同じ意味で揃える |
| 既存パターン優先 | 新しい形を足す前に、同種の実装を検索して合わせる |
| 検証 | Go の変更後は原則 `go test ./...` を通す |

## レイヤー境界

ルール:

- ドメイン層は状態、不変条件、ドメイン操作だけを持つ
- ユースケース層は入力を受け、解決済みの依存を使って処理順序を表す
- アダプター層は外部システム、ファイル、コマンド、永続化形式の詳細を扱う
- アプリケーション層は CLI 解析、依存の組み立て、標準入出力を扱う
- 下位の業務概念が上位の実装詳細を知らないようにする
- テストの都合だけで本番 API を歪めない

例:

- `domain` から外部コマンド、ファイルシステム、Docker、tmux、git の詳細に依存しない
- `usecase` に shell command の組み立てや YAML/JSON の低レベル処理を置かない
- `adapter` に業務判断や実行順序の判断を置かない
- `app` に業務ルールを置かない
- 外部コマンドは runner など既存の境界を通す
- structured data は構造体と parser を使い、文字列処理だけで扱わない

## エラーハンドリング

ルール:

- 失敗を成功扱いにしない
- 呼び出し元が判断できる情報を落とさない
- 文脈が必要な境界では error を wrap する
- 利用者に見えるエラーは、何が失敗したか分かる文言にする
- `_ = err` や空の error branch は原則使わない

例:

```go
// REJECT: エラーを握りつぶして成功扱いにする
func load(path string) Config {
	cfg, err := readConfig(path)
	if err != nil {
		return Config{}
	}
	return cfg
}

// OK: 失敗を呼び出し元へ返す
func load(path string) (Config, error) {
	cfg, err := readConfig(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
```

## 入力検証とデフォルト値

ルール:

- 必須値を暗黙のデフォルトで埋めない
- 設定ミス、呼び出し漏れ、未解決値は失敗させる
- optional な値だけが default を持てる
- default を使う場合は仕様、設定、テストで明示する
- 後方互換のための fallback は範囲と除去条件を明示する

例:

```go
// REJECT: 必須値が空でも処理が進む
func newSession(name string) Session {
	if name == "" {
		name = "default"
	}
	return Session{Name: name}
}

// OK: 必須値は検証する
func newSession(name string) (Session, error) {
	if name == "" {
		return Session{}, errors.New("session name is required")
	}
	return Session{Name: name}, nil
}
```

## 解決責務

ルール:

- 実行前に確定できる値は、境界で一度だけ解決する
- 同じ優先順位ロジックを複数層に置かない
- 下位層には raw config ではなく、解決済みの値を渡す
- 表示、実行、保存で同じ値を使う場合は、同じ解決結果を共有する
- 解決処理と副作用実行を混ぜない

例:

```go
// REJECT: 呼び出し側と実装側の両方で同じ値を補完する
func (u UseCase) Execute(ctx context.Context, input Input) error {
	provider := input.Provider
	if provider == "" {
		provider = u.Config.Provider
	}
	return u.Factory.Source(domain.ProviderConfig{Source: provider})
}

func (f Factory) Source(provider domain.ProviderConfig) (SourcePort, error) {
	if provider.Source == "" {
		provider.Source = "git"
	}
	// ...
}

// OK: 上位で解決し、下位は解決済み値だけを使う
func (u UseCase) Execute(ctx context.Context, input Input) error {
	providers, err := u.Config.LoadProviders(ctx)
	if err != nil {
		return err
	}
	source, err := u.Factory.Source(providers)
	if err != nil {
		return err
	}
	return source.CreateSource(ctx, input.Cell)
}
```

## フェーズ分離

ルール:

- 入力収集、解釈、正規化、検証、実行、副作用、出力を分ける
- ループ内で毎回 raw input を解釈し直さない
- 実行フェーズには実行に必要な値だけを渡す
- エラーの発生箇所と処理の責務を近づける

例:

```go
// REJECT: ループ内で入力解釈と実行が混ざる
for _, item := range items {
	name := item.Name
	if name == "" {
		name = "default"
	}
	if err := runner.Run(ctx, "tool", "create", name); err != nil {
		return err
	}
}

// OK: 先に正規化し、ループ内は実行だけ
resolved, err := resolveItems(items)
if err != nil {
	return err
}
for _, item := range resolved {
	if err := createItem(ctx, runner, item); err != nil {
		return err
	}
}
```

## 抽象化

ルール:

- 抽象化は概念に名前を与え、変更理由を揃えるために使う
- 表面的な重複だけで共通化しない
- 1箇所だけの処理を無理に設定テーブルや関数オブジェクトへ変換しない
- interface や関数名は利用側の概念で説明できる名前にする
- 抽象化で処理順序やエラー箇所が読みにくくなるなら避ける

例:

```go
// REJECT: 1箇所しか使わない設定テーブルで副作用が読みにくい
var actions = []struct {
	name string
	fn   func(context.Context, Cell) error
}{
	{"source", createSource},
	{"container", createContainer},
	{"session", createSession},
}
for _, action := range actions {
	if err := action.fn(ctx, cell); err != nil {
		return err
	}
}

// OK: usecase の orchestration は順序を明示する
if err := source.CreateSource(ctx, cell); err != nil {
	return err
}
if err := containers.CreateContainers(ctx, cell, template); err != nil {
	return err
}
if err := session.CreateSession(ctx, cell); err != nil {
	return err
}
```

## インターフェース設計

ルール:

- interface は利用側が必要とする操作を表す
- 実装側の内部構造や外部ツール名を API に漏らさない
- 構成、検証、実行、削除など異なる責務を混ぜない
- 同じ意味の操作を複数メソッドに分裂させない
- 引数は呼び出し側が自然に持っている単位にする

例:

```go
// REJECT: 実装詳細ごとにメソッドが増える
type SessionPort interface {
	CreateTmuxSession(ctx context.Context, cell domain.Cell) error
	CreateTmuxWindow(ctx context.Context, cell domain.Cell, window domain.SessionWindow) error
}

// OK: usecase が必要とする操作だけを表す
type SessionPort interface {
	CreateSession(ctx context.Context, cell domain.Cell) error
	RemoveSession(ctx context.Context, cell domain.Cell) error
	EnterSession(ctx context.Context, cell domain.Cell) error
}
```

## 契約変更

ルール:

- 型、interface、設定スキーマ、永続化形式、CLI、ファイル形式は同じ契約として扱う
- 契約を変えたら、定義側、生成側、利用側、検証側を同じ変更で揃える
- 利用者に見える契約は README、サンプル設定、テストも更新する
- 既存データとの互換性が必要な場合は migration または load test を用意する

例:

- field を追加したら config load/save、factory、usecase、test を確認する
- interface を変えたら adapter、fake、test を同時に更新する
- 設定値を追加したらサンプル YAML と validation を更新する
- 永続化形式を変えたら既存形式の読み込みテストを追加する

## 状態管理

ルール:

- 同じ事実を複数の状態として保持しない
- 正規状態から計算できる値は派生値として扱う
- 状態変更は domain method など不変条件を守れる場所に寄せる
- 外部形式との変換は marshal/unmarshal など境界で行う

例:

```go
// REJECT: Done と Status の同期が必要になる
type Cell struct {
	Done   bool
	Status string
}

// OK: 正規状態を1つにする
type Cell struct {
	done bool
}

func (c Cell) IsDone() bool {
	return c.done
}
```

## テスト

ルール:

- 新しい挙動は該当層のテストを先に更新する
- 外部コマンド、ファイル、ネットワークなどの副作用は境界越しに検証する
- usecase は fake port で呼び出し順と状態変更を検証する
- adapter は実行引数、parser、外部形式の変換を検証する
- domain は不変条件と状態変更を検証する
- app は CLI parse と wiring を検証する
- 最後に `go test ./...` を通す

## 未完成コード

ルール:

- TODO/FIXME、空実装、スタブ、コメントアウトされた旧実装を完成した実装の代わりに残さない
- やむを得ず残す場合は、理由、除去条件、追跡先を明記する
- 使われなくなった helper、型、設定、テスト fixture は同じ変更で整理する

例:

- `return nil` だけの空実装を残さない
- エラー処理や validation を TODO で先送りしない
- 置き換え済みの旧コードをコメントアウトで残さない

## 機密情報

ルール:

- パスワード、トークン、API キー、セッション ID、認証ヘッダ、個人情報をコードやログに露出させない
- request/DTO 全体を不用意にログ出力しない
- テスト fixture には実トークンや実パスを含めない
- エラーメッセージは原因を示しつつ、秘密値を含めない

## 共通化の判断

ルール:

- 同じ理由で変わる処理はまとめる
- 似ていても変更理由が違う処理は分けてよい
- 新規関数の本体が既存関数と酷似していないか確認する
- 共通化すると呼び出し側の意味が曖昧になる場合は名前を見直す

例:

```go
// REJECT: 同じ実装が別名で存在
func copyFiles(src, dst string) error {
	return copyTree(src, dst)
}

func placeFiles(src, dst string) error {
	return copyTree(src, dst)
}

// OK: 呼び出し元を1つの名前へ統一する
func copyFiles(src, dst string) error {
	return copyTree(src, dst)
}
```

## 汎用禁止ルール

- 不正な入力や未解決値を暗黙に補完して処理を続けない
- 失敗を握りつぶさない
- 仕様にない副作用や外部依存を層の内側へ持ち込まない
- 同じ契約、キー、状態、優先順位を複数箇所で独立管理しない
- 使われないコード、未完成コード、古い実装を残さない
- 問題の根本原因を直さず、呼び出し側の迂回で隠さない
- 依頼範囲外の rename、format、refactor を混ぜない
