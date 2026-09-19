# ディレクトリ構成ルール

- 1つのファイルには、原則として1つの型だけを定義すること。
- 同じ利用側packageに属する関連Portは、`ports.go`へまとめてよい。
- 型は、その責務が分かる名前のファイルへ置くこと。
- constructorを定義する場合は、その型を定義したファイルへ置くこと。
- その型だけに属するmethodは、型を定義したファイルへ置くこと。
- interfaceは、そのinterfaceを必要とする利用側packageへ置くこと。
- Domain Serviceが必要とするPortはdomain layerへ置くこと。
- Application Serviceだけが必要とするPortはapplication layerへ置くこと。
- 別packageのinterfaceをtype aliasして、interfaceの所有場所を曖昧にしないこと。
- Adapterは、実装対象のinterfaceを所有する内側のlayerへ依存すること。
- `.codex`配下のルールは`.codex/rules`へ置き、`instructions.md`を読込入口とすること。
