# コーディングルール

## 最小実装

- 現在のcaller、test、interface、または明文化された不変条件に必要な処理だけを実装すること。
- 現在の用途がない型、関数、method、field、interface、fallback、互換処理を追加しないこと。
- 新しいsymbolを追加する前に、現在のcallerまたは具体的な不変条件を確認すること。どちらもなければ追加しないこと。
- genericなSummary、Display、wrapper、forwarding methodを作らないこと。
- 直接書ける処理を、意味のないhelperで包まないこと。
- 実装完了前に`deadcode ./...`相当の解析を行い、到達不能なproduction関数を残さないこと。
- 未使用処理だけを検証するtestは、未使用処理と一緒に削除すること。

## Constructor

- user-definedな具象型には`NewXxx` constructorを定義し、その型を生成するときはconstructorを使うこと。
- named primitive typeは、許可する値をtyped constantで定義すること。
- named primitive typeのconstructorは入力値を検証し、外部入力を直接castしないこと。
- defaultを持つ型は、不正値を明文化されたdefaultへ変換すること。
- 不正値を拒否する型は、constructorからerrorを返すこと。
- 検証済みの値を各layerで重複検証しないこと。

## AggregateとEntity

- Aggregate外から、Aggregate Rootまたは所有Entityのfieldを直接参照・変更しないこと。
- Aggregateが所有するEntityの問い合わせと変更は、目的を表すAggregate Rootのmethodを通すこと。
- Aggregate RootまたはEntity自身に属する振る舞いは、その型のmethodとして定義すること。
- receiverを変更するmethodはポインタレシーバにし、既存instanceを更新すること。
- 構造体を変更する処理を、値を受け取って変更後のcopyを返すfree functionとして定義しないこと。
- 同一性判定や状態変更を、fieldごとのgetterへ分解しないこと。

## Value Object

- EntityではないValue Objectはstructとして定義すること。
- Value Objectのconstructorは、同じファイルで新しいstruct値を生成して返すこと。
- Value Objectを変更する操作は値レシーバのmethodにし、元の値を変更せず、新しい値をconstructor経由で返すこと。
- 値をそのまま返すだけのgetterや`String` methodを追加しないこと。

## Domain ServiceとPort

- 単一のAggregate RootやEntityへ属さない処理だけを、独立したDomain Serviceとして定義すること。
- Domain ServiceをAggregate Rootのforwarding methodとして追加しないこと。
- Application Serviceが必要なPortをFactoryから生成し、入力値、Port、更新対象Aggregate RootのポインタをDomain Serviceへ渡すこと。
- interfaceは実装側ではなく利用側に定義し、利用側が必要とするmethodだけを含めること。
- Application Serviceだけが使う永続化Portをdomain packageへ置かないこと。

## 変更後の確認

- 変更したコードについて、requestとrepositoryの事実に反する仮定がないか確認すること。
- 存在しないAPI、method、command、設定を追加していないか確認すること。
- scope creep、不要なfallback、互換処理、重複validation、未使用helper、callerのないsymbolを削除すること。
- 近接コードの命名と構造を確認し、repositoryに存在しない一般論のarchitectureを持ち込まないこと。
- formatter、test、race test、static analysis、diff checkを実行すること。
