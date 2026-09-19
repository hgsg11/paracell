# ディレクトリ構成ルール

- 型を定義するdomain objectは、1つの型につき1ファイルへ分けること。
- aggregate、entity、value object、Port用DTO、Templateは、それぞれ責務が分かる名前のファイルへ置くこと。
- constructorと、その型だけに属するmethodは、型を定義したファイルへ置くこと。
- 複数の型を、便宜上1つのファイルへまとめないこと。
- interfaceは、そのinterfaceを必要とする利用側packageへ置くこと。
- Domain Serviceが外部resource操作に必要とするPortは`internal/domain`へ置くこと。
- Usecaseだけが永続化や設定読込のために必要とするPortは`internal/usecase`へ置くこと。
- 別packageのinterfaceをtype aliasして、interfaceの所有場所を曖昧にしないこと。
- Adapterは、実装するinterfaceが置かれた内側のpackageへ依存すること。
- `.codex`配下のルールは`.codex/rules`へ置き、`instructions.md`を読込入口とすること。
