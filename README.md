# paracell

issue ごとに、git worktree・tmux session・container・状態管理をまとめて作る CLI です。

`paracell fork 123 --template feat` で issue 用の作業部屋を作り、`paracell` で project 用 root tmux session に入り、そこから cell を見る・入る・片付ける。agent の hooks から `paracell pending` / `paracell ready` を呼べば、作業状態も自動で動きます。

## できること

- 🧩 issue ごとの git worktree と branch を作る
- 🪟 tmux session / window / 初期コマンドを template 化する
- 📦 container、volume、network、`.env` などを cell ごとに用意する
- 👀 `ready` / `pending` / `done` を TUI で見る
- 🤖 agent hooks から状態を自動更新する

## ユースケース

| ユースケース | paracell なし | paracell あり |
| --- | --- | --- |
| 🛠️ ローカル開発環境を作る | branch、作業ディレクトリ、tmux、container を手でそろえる | `paracell fork <issue>` で cell としてまとめて作る |
| 🔍 レビュー用に動作確認する | レビュー対象ごとに checkout や環境切り替えをする | review 用 cell を作り、今の作業環境と分けて確認する |
| 🧯 レビュー指摘を修正する | 元の作業環境に戻り、必要な window や service を開き直す | 対象 cell に入り直して、そのまま修正を続ける |
| 🤖 agent に修正を任せる | 進行中か完了かをログやメモで追う | hooks で `pending` / `ready` を呼び、TUI に状態を出す |
| 🧹 作業環境を片付ける | worktree、tmux session、container を個別に消す | `paracell clean <cell>` でまとめて片付ける |

## インストール

```sh
brew install --cask hgsg11/homebrew-paracell/paracell
```

Nix を使う場合:

```sh
nix run github:hgsg11/paracell
# または
nix profile install github:hgsg11/paracell
```

Nix package には実行時依存として `git` と `tmux` が含まれます。

リリース前に `./scripts/set-release-version.sh vX.Y.Z` を実行して、`VERSION` の変更をリリース対象へ含めます。リリースワークフローはタグと `VERSION` が一致しない場合に停止します。

または:

```sh
go build -o paracell ./cmd/paracell
```

## クイックスタート

```sh
paracell init
paracell fork 123 --template feat
paracell
```

`paracell init` は `paracell.yaml` を作ります。template を編集して、作りたい cell の形を決めます。

```yaml
project:
  name: ""
providers:
  source: git
  session: tmux
  notifications: tmux
templates:
  feat:
    repository:
      branchPrefix: feat/
      base: main
    session:
      windows: []
  update:
    repository:
      branchPrefix: update/
      base: main
    session:
      windows: []
  fix:
    repository:
      branchPrefix: fix/
      base: main
    session:
      windows: []
  review:
    repository:
      branchPrefix: review/
      base: main
    session:
      windows: []
```

## TUI

```text
  NAME  TEMPLATE  STATUS   DONE
> 123   feat      ready    [ ]
  456   fix       pending  [x]

  go root
```

- `j` / `k`: 移動
- `tab`: cell 一覧と template 一覧のフォーカス切り替え
- `l`: cell に入る
- `enter`: done を切り替える
- `d` `d`: clean
- template 一覧で `y` `y`: issue 番号入力モード
- issue 番号入力後 `enter`: fork
- `q`: 閉じる

tmux の中で `paracell pending` / `paracell ready` を実行すると、現在の cell の `STATUS` が変わります。`view` は自動で state を読み直します。

`paracell ready` は `Ready: {{.name}}` を `tmux display-message` で表示します。通知は `providers.notifications: tmux` のときだけ有効です。

`paracell` を引数なしで実行すると、project ごとの root tmux session に入ります。そこで `C-p` を押すと `paracell view` を popup で開けます。

paracell が作成した tmux session の中では、`C-p` で `paracell view` を popup で開けます。popup から `l` を押すと、選択した cell の tmux session に切り替わります。

paracell の root / cell tmux session ではマウス操作が有効です。ドラッグした文字列は tmux のバッファにコピーされ、OSC 52 対応の terminal では system clipboard にも反映されます。ホイールで copy mode に入り、履歴をスクロールできます。

tmux session 名は、root が `<project>-root`、cell が `<project>-<issue>` です。

paracell が管理する tmux session では、ターミナルのタブタイトルを `<project>` に固定します。ステータスラインの window 表示は、root session が `root:<window>`、cell session が `<issue>:<window>` です。右側には時刻と日付を表示し、pane タイトルは表示しません。それ以外のステータスライン設定は tmux の現在の設定を引き継ぎます。

`C-p` の popup は幅65列、高さ50%で表示され、tmux の `display-popup` 対応版を前提にしています。

## コマンド

```text
paracell init
paracell fork <issue> --template <template> [--command <command>]
paracell view
paracell ls
paracell clean <cell> [--force]
paracell pending
paracell ready
paracell exit
paracell version
paracell --version
```

- `fork`: issue 用の cell を作る。`--command` で template に渡す初期命令を指定できる
- `view`: TUI で cell を操作する
- `ls`: cell 一覧を出す
- `clean`: cell の worktree / container / session を片付ける
- `pending` / `ready`: `PARACELL_CELL` の status を変える
- `exit`: tmux client を detach し、`paracell` を実行した元のシェルとディレクトリに戻る

## 設定メモ

- `repository.base: current`: 現在の branch から cell branch を作る
- `repository.base: main`: `main` から cell branch を作る
- `repository.branchMode: create`: cell branch を新規作成する。既存 branch があれば失敗する
- `repository.branchMode: reuse`: 既存 branch があればその branch の worktree を作り、なければ新規作成する
- `repository.branchMode: require`: 既存 branch の worktree だけを作る。branch がなければ失敗する
- `files`: cell の source にコピーするファイル
- `containers.network: isolated`: cell 用 Docker network を作る
- `containers.network: shared`: source container の network を使う
- `volumeMode: copy`: named volume を複製する
- `volumeMode: readonly`: 共有 volume を read-only で使う
- database service の `volumeMode` は `copy` のみ対応する
- `database.copyMode: schema`: system database を除く全 DB の schema を cell に用意する
- `database.copyMode: data`: 予約済み。まだ未実装
- `providers.notifications: tmux`: `paracell ready` 後に tmux message を出す
tmux command では `{{.issue}}`、`{{.name}}`、`{{.Command}}` を使えます。`{{.Command}}` は `fork --command` で指定した初期命令へ展開されます。TUI から fork した場合は空文字列です。

## ファイル

- `paracell.yaml`: 設定と template
- `.paracell/state.json`: cell の状態
- `.paracell/cells/<cell>/source`: cell の git worktree
