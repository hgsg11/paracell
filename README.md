# paracell

**AI agent ごとに、issue 単位の独立した開発環境を。**

Paracell は、git worktree・tmux session・container・network・状態管理を、ひとつの **cell** としてまとめて作る CLI です。複数の AI agent を同じ repository で動かしても、branch、作業ディレクトリ、依存 service、port を issue ごとに分離できます。

```sh
paracell fork 123 --template feat
```

この1コマンドで、issue #123を進めるための独立した作業環境を起動できます。

## AI agent の並行開発を、環境ごと分離する

AI agent を並行実行すると、同じファイル、同じcontainer名、同じhost portを取り合いやすくなります。Paracellは作業単位をcellとして分け、agentに「専用のrepository checkoutと実行環境」を渡します。

### 1 issue = 1 cell

| Cellに含まれるもの | 分離される内容 |
| --- | --- |
| Source | issue用branchとgit worktree |
| Terminal | 専用tmux session、window、agentの初期コマンド |
| Runtime | container、volume、環境変数、Docker network |
| Endpoint | cell名を含む `.localhost` URL |
| State | 作成状態 `creating` / `failed` / `ready`、作業状態 `pending` / `ready`、`done` と実行ログ |

templateへCodexなどのagent起動コマンドを設定すれば、cell作成と同時にagentへ作業を渡せます。agentのhookから `paracell pending` / `paracell ready` を呼び、人間はTUIから複数cellの状態を確認できます。

### Traefikで、issueごとの通信を調査する

isolated networkを使うcellには、共有Traefik gatewayが自動で接続されます。frontendとbackendが同じcontainer portを使っていても、issueとservice aliasを含むURLで衝突せずアクセスできます。

```text
http://frontend.123.myapp.localhost
http://backend.123.myapp.localhost
```

Traefik dashboardでrouterと接続先を確認し、access logで「どのissueのURLが、どのcontainerへ流れたか」を追跡できます。Prometheus metricsとOpenTelemetry tracingも利用できます。

```sh
open http://gateway.paracell.localhost/dashboard/
docker logs -f paracell-gateway
curl http://gateway.paracell.localhost/metrics
```

cellを使い終えたら、worktree、tmux session、container、networkをまとめて片付けます。

```sh
paracell clean 123
```

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

cell の用途を短く示したい場合は、fork 時または作成後に note を設定できます。

```sh
paracell fork 123 --template feat --note "PostgreSQL案"
paracell annotate 123 --note "API実装中"
```

`paracell init` は `paracell.yaml` を作ります。template を編集して、作りたい cell の形を決めます。

`fork` が source、files、containers、session の途中で失敗した場合、cell は `failed` として残り、完了済み工程と branch/worktree は保持されます。原因を修正してから同じ cell を再開してください。

```sh
paracell ls
paracell retry 123
```

`retry` は ID、Issue、Name のいずれでも cell を指定できます。同じcellのretryは一度に1実行だけで、実行中に重ねて呼ぶと待機せず `retry already in progress` エラーになります。retryプロセスが異常終了した場合は、最終heartbeatから2分を超えると保存済みcheckpointから再取得できます。最新の `paracell.yaml` で失敗工程以降を再renderし、完了済み工程は再作成しません。files工程では、worktree内に同内容のファイルがあれば再利用し、内容が異なる既存ファイルはユーザー変更を守るため上書きせず失敗します。

```yaml
project:
  name: ""
providers:
  source: git
  session: tmux
  notifications: tmux
templates:
  base:
    abstract: true
    repository:
      base: main
      branchMode: create
    session:
      windows:
        - name: agent
          command: 'codex "{{.Command}}"'
  feat:
    extends: base
    repository:
      branchPrefix: feat/
  update:
    extends: base
    repository:
      branchPrefix: update/
  fix:
    extends: base
    repository:
      branchPrefix: fix/
  review:
    extends: base
    repository:
      branchPrefix: review/
```

template は `extends` で一つの親を継承できます。親自身も別のtemplateを継承できます。共通設定だけを持つtemplateには `abstract: true` を付けます。abstract templateは継承元には使えますが、`fork --template` とTUIの選択肢には表示されません。

継承では、子で未指定のscalarとstruct fieldは親の値を維持し、子で指定した値は空文字を含めて上書きします。`files`、`session.windows`、`containers.services`のようなslice/mapは、子で指定するとcollection全体を置換します。親子の要素はappend/deep mergeされません。`[]`または`{}`を指定すると明示的に空へ置換できます。

存在しない親、自己参照、循環参照は設定読込時にエラーになります。循環エラーには `"a" -> "b" -> "a"` のように参照経路が含まれます。継承されたtemplate変数は、選択した具体templateの `issue`、`name`、`project`、`Command` で展開され、その後に通常の設定validationが実行されます。

## Isolated container gateway

Docker provider で `containers.network: isolated` を使うと、paracell は共有の `paracell-gateway` container（Traefik）を用意し、host の `127.0.0.1:80` だけに公開します。gateway は各 cell 専用 network へ接続され、`paracell.yaml` の `containers.services` map key（service role）と公開済み TCP container port を使って route を自動生成します。gateway 用の設定を `paracell.yaml` に追加する必要はありません。

Traefik dashboard はデフォルトで有効になり、次の URL から利用できます。末尾の `/` は必須です。dashboard と API は専用の管理 port や `api.insecure` を使わず、既存の loopback-only web entrypoint を通じて `gateway.paracell.localhost` にだけ route されます。

```text
http://gateway.paracell.localhost/dashboard/
```

access log、tracing、Prometheus metrics もデフォルトで有効です。metrics は別portを公開せず、同じhostの `http://gateway.paracell.localhost/metrics` から取得できます。

通信を調査するときは、用途に応じて次を使います。

```sh
# router、service、middleware、設定errorを確認する
open http://gateway.paracell.localhost/dashboard/

# requestのHost、path、status、接続先、処理時間を追う
docker logs -f paracell-gateway

# Prometheus形式の現在値を取得する
curl http://gateway.paracell.localhost/metrics
```

dashboardはrouting設定を確認する画面で、access logや時系列graphの保存先ではありません。metricsの履歴とgraphが必要ならPrometheus / Grafana、traceの検索画面が必要ならOpenTelemetry CollectorとJaeger / Tempoなどを接続してください。

公開 port が 1 個の container は、すべての HTTP path と WebSocket を次の URL で利用できます。

```text
http://<service-role>.<cell>.<project>.localhost
```

公開 port が複数ある場合は、container port ごとに prefix が付きます。

```text
http://p<containerPort>.<service-role>.<cell>.<project>.localhost
```

たとえば project `myapp` の cell `123` で service role `frontend` が container port `3000` と `8080` を公開していれば、`http://p3000.frontend.123.myapp.localhost` と `http://p8080.frontend.123.myapp.localhost` を使います。source container名、Composeが生成したalias、追加のnetwork aliasはURLになりません。route の upstream はcell固有のcopied containerとinternal portです。そのため、複数cellが同じservice roleを持っていても衝突しません。公開TCP portがないcontainerにはrouteを作りません。

source network aliasは、copied containerから`http://backend`のようにcontainer間通信するため、isolated networkへ引き続きコピーされます。さらにcopied containerにはservice role自身もnetwork aliasとして必ず追加されます。これらの内部通信用aliasと、外部公開するcanonical URLは別のものです。

Paracellが作成するDocker resource名は、networkが`paracell-<project>-<cell>`、copied containerが`paracell-<project>-<cell>-<service-role>`です。ユーザーが起動したsource containerの名前は変更せず、このmanaged resource命名の対象にもなりません。project名自体が`paracell`なら`paracell-paracell-...`となりますが、先頭の`paracell`は管理namespace、2つ目はproject名であり、重複を省略しません。既存cellのresourceやURLは自動renameされず、修正版でcellを再作成した時点からこの規則が適用されます。

`paracell clean` は copied container の削除によって route を解除し、gateway を cell network から切断してから network を削除します。gateway 自体はほかの project / cell でも共有するため残ります。`containers.network: shared` の動作は従来どおりで、gateway の対象外です。

初回起動時には Docker が `traefik:v3.7` image を利用できる必要があります。まず `127.0.0.1:80` を使い、port 80 が別 process/container に使われていれば loopback 上の空き port を Docker に自動割当させます。fallback 先は `docker port paracell-gateway 80/tcp` で確認でき、その場合は表示された port を URL へ付けてください。たとえば fallback port が `49152` なら dashboard は `http://gateway.paracell.localhost:49152/dashboard/` です。gateway は route 検出のため `/var/run/docker.sock` を read-only mount しますが、Docker API socket 自体が強い権限を持つ点には注意してください。

## TUI

```text
  CELL              TEMPLATE  STATUS   DONE
> 123 | API実装中   feat      ready    [ ]
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

TUI から実行した Git、Docker、tmux などの標準出力・標準エラーと Paracell 自身のエラーは、画面下部の共通ログ領域へリアルタイム表示されます。長い行と複数行のエラーは画面幅で折り返され、表示は常に最新行へ追従します。

TUI と単独 CLI のどちらから実行しても、コマンドの開始・成功・失敗、Paracell の標準出力、および外部処理の標準出力・標準エラーは、project ごとの `.paracell/logs/paracell-YYYYMMDD.log` に `時刻 レベル [処理元] 内容` のプレーンテキスト形式で保存されます。単独 CLI の標準出力・標準エラーは従来どおり端末にも表示されます。

ログは日付単位でファイルを分け、複数の CLI/TUI プロセスから同じ日のファイルへ追記できます。過去の日次ログは自動削除しません。`.paracell/` は Git 管理外です。

tmux の中で `paracell pending` / `paracell ready` を実行すると、現在の cell の `STATUS` が変わります。`view` は自動で state を読み直します。

`paracell ready` は `Ready: {{.name}}` を `tmux display-message` で表示します。通知は `providers.notifications: tmux` のときだけ有効です。

`paracell` を引数なしで実行すると、project ごとの root tmux session に入ります。そこで `C-p` を押すと `paracell view` を popup で開けます。

paracell が作成した tmux session の中では、`C-p` で `paracell view` を popup で開けます。popup から `l` を押すと、選択した cell の tmux session に切り替わります。

paracell の root / cell tmux session ではマウス操作が有効です。ドラッグした文字列は tmux のバッファにコピーされ、OSC 52 対応の terminal では system clipboard にも反映されます。ホイールで copy mode に入り、履歴をスクロールできます。

tmux session 名は、root が `<project>-root`、cell が `<project>-<issue>` です。

paracell が管理する tmux session では、ターミナルのタブタイトルを `<project>` に固定します。ステータスラインの左側は、root session が `root`、cell session が note（未設定なら `<issue>`）です。window 表示も note（未設定なら `<issue>`）を label に使います。右側は tmux の既存表示を保ちながら時刻と日付を追加します。それ以外のステータスライン設定は tmux の現在の設定を引き継ぎます。note を更新すると、起動中の session へ即時反映されます。

### PC 再起動後に tmux session を復元する

[tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) と [tmux-continuum](https://github.com/tmux-plugins/tmux-continuum) を使う場合は、たとえば TPM の設定を次のようにします。continuum は自動保存用の処理を `status-right` に追加するため、plugin 一覧の最後に置いてください。paracell は既存の `status-right` を残したまま時刻と日付を追加します。

```tmux
set -g @plugin 'tmux-plugins/tmux-resurrect'

# Codex が動いていた pane は、保存時のコマンドをそのまま再実行せず
# その作業ディレクトリで最後の会話を再開する。
set -g @resurrect-processes '"~codex->codex resume --last"'

set -g @continuum-restore 'on'
set -g @plugin 'tmux-plugins/tmux-continuum'

# TPM の初期化は plugin 設定より後に置く。
run '~/.tmux/plugins/tpm/tpm'
```

continuum が tmux server 起動時に通常の resurrect 復元を行います。復元後は従来どおり `paracell` から root session に入り、TUI から cell に入ってください。その際、resurrect が保存対象にしない `PARACELL_ROOT` / `PARACELL_CELL`、paracell key table、new-window hook、復元済み全 window の label を再設定します。復元時点ですでに起動済みの shell は後から session 環境変数を継承できないため、`pending` / `ready` は `PARACELL_CELL` がなくても `.paracell/cells/<cell>/source` 以下の作業ディレクトリから cell を判定します。

これらの plugin や設定がない場合、paracell の session 作成・接続動作は従来どおりです。独自の保存・復元コマンドは追加していません。

`C-p` の popup は幅65列、高さ24行の固定サイズで画面中央に表示され、tmux の `display-popup` 対応版を前提にしています。

## コマンド

```text
paracell init
paracell fork <issue> --template <template> [--command <command>] [--note <note>]
paracell annotate <cell> --note <note>
paracell retry <cell>
paracell view
paracell ls
paracell clean <cell> [--force]
paracell pending
paracell ready
paracell exit
paracell version
paracell --version
```

- `fork`: issue 用の cell を作る。`--command` で template に渡す初期命令、`--note` で表示用の短い説明を指定できる。option の順序は任意
- `annotate`: ID、Issue、Name のいずれかで既存 cell を指定し、note を設定・上書きする
- `retry`: failed cellをID、Issue、Nameで指定し、cell単位の排他leaseを取得して保存済みcheckpointから作成を再開する
- `view`: TUI で cell を操作する
- `ls`: cell一覧と作成状態、work status、done状態を出す。failed cellでは失敗工程と直近errorも1行で表示する
- `clean`: cell の worktree / container / session を片付ける
- `pending` / `ready`: `PARACELL_CELL` の status を変える
- `exit`: tmux client を detach し、`paracell` を実行した元のシェルとディレクトリに戻る

note は前後・改行・tab・連続空白を単一 space に正規化した後、Unicode で1〜20文字である必要があります。`paracell ls` と tmux は note を cell 名より優先し、TUI は `<cell名> | <note>` と併記します。note は表示専用であり、cell の指定には引き続き ID、Issue、Name を使います。branch、worktree、container、network、session 名も変わりません。

## 設定メモ

- `extends`: 一つの親templateを指定する。複数段継承は可能だが多重継承は不可
- `abstract: true`: 共通設定用templateとして宣言し、CLI/TUIの選択肢から除外する
- `repository.base: current`: 現在の branch から cell branch を作る
- `repository.base`: cell branch の作成元を指定する。`main` や `feature/111` など任意の branch を指定できる
- `repository.branchMode: create`: cell branch を新規作成する。既存 branch があれば失敗する
- `repository.branchMode: reuse`: 既存 branch があればその branch の worktree を作り、なければ新規作成する
- `repository.branchMode: require`: 既存 branch の worktree だけを作る。branch がなければ失敗する
- `files`: cell の source にコピーするファイル
- `containers.network: isolated`: cell 用 Docker network と自動 HTTP gateway route を作る
- `containers.network: shared`: source container の network を使う
- `containers.services.<service>.environment`: service ごとの環境変数を設定する。source container の環境変数をコピーした後、同名の変数をこの設定で上書きする
- `volumeMode: copy`: named volume を複製する
- `volumeMode: readonly`: 共有 volume を read-only で使う
- database service の `volumeMode` は `copy` のみ対応する
- `database.copyMode: schema`: system database を除く全 DB の schema を cell に用意する
- `database.copyMode: data`: 予約済み。まだ未実装
- `providers.notifications: tmux`: `paracell ready` 後に tmux message を出す
tmux command では `{{.issue}}`、`{{.name}}`、`{{.Command}}` を使えます。`{{.Command}}` は `fork --command` で指定した初期命令へ展開されます。TUI から fork した場合は空文字列です。

container service の環境変数では `{{.issue}}`、`{{.name}}`、`{{.project}}` を使えます。`environment` にない変数は source container の値をそのまま引き継ぎ、空文字列を指定した変数は明示的に空へ上書きします。isolated service でも environment は application container に適用され、共有 gateway の設定と route はそのまま維持されます。

```yaml
containers:
  network: isolated
  services:
    web:
      sourceContainer: myapp-web
      environment:
        PARACELL_ISSUE: "{{.issue}}"
        PARACELL_CELL: "{{.name}}"
        PARACELL_PROJECT: "{{.project}}"
        OPTIONAL_VALUE: ""
```

## ファイル

- `paracell.yaml`: 設定と template
- `.paracell/state.db`: cell の状態を保存するSQLite database
- `.paracell/cells/<cell>/source`: cell の git worktree

旧形式の `.paracell/state.json` は読み込みません。
