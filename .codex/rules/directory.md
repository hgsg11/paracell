# ディレクトリ構成ルール

- 1つのファイルには、原則として1つの型だけを定義すること。Domain Service専用Portは、そのServiceと同じファイルに定義してよい。
- 同じ利用側packageに属する関連Portは、`ports.go`へまとめてよい。
- Domain Serviceは`Service`接尾辞を付けた名前にし、対応するsnake_caseファイル（例: `CreateContainersService`なら`create_containers_service.go`）へ1関数だけ定義すること。
- 型は、その責務が分かる名前のファイルへ置くこと。
- constructorを定義する場合は、その型を定義したファイルへ置くこと。
- その型だけに属するmethodは、型を定義したファイルへ置くこと。
- interfaceは、そのinterfaceを必要とする利用側packageへ置くこと。
- UseCaseが使うPortはusecase packageへ置くこと。Domain Serviceが使うPortはdomain packageの当該Serviceと同じファイルへ置くこと。
- 別packageのinterfaceをtype aliasして、interfaceの所有場所を曖昧にしないこと。
- Adapterは、実装対象のinterfaceを所有する内側のlayerへ依存すること。
- `.codex`配下のルールは`.codex/rules`へ置き、`instructions.md`を読込入口とすること。
