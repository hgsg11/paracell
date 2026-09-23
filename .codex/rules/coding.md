# コーディングルール

## 最小実装

- 現在のcaller、test、interface、または明文化された不変条件に必要な処理だけを実装すること。
- 現在の用途がない型、関数、method、field、interface、fallback、互換処理を追加しないこと。
- 新しいsymbolを追加する前に、現在のcallerまたは具体的な不変条件を確認すること。どちらもなければ追加しないこと。
- genericなSummary、Display、wrapper、forwarding methodを作らないこと。
- 直接書ける処理を、意味のないhelperで包まないこと。
- 実装完了前に`deadcode ./...`相当の解析を行い、到達不能なproduction関数を残さないこと。
- 未使用処理だけを検証するtestは、未使用処理と一緒に削除すること。

## AggregateとEntity

- Aggregate外から、Aggregate Rootまたは所有Entityのfieldを直接参照・変更しないこと。
- Aggregateが所有するEntityの問い合わせと変更は、目的を表すAggregate Rootのmethodを通すこと。
- Aggregate RootまたはEntity自身に属する振る舞いは、その型のmethodとして定義すること。
- receiverを変更するmethodはポインタレシーバにし、既存instanceを更新すること。
- 構造体を変更する処理を、値を受け取って変更後のcopyを返すfree functionとして定義しないこと。
- 同一性判定や状態変更を、fieldごとのgetterへ分解しないこと。
- Aggregate Root、Entity、Value ObjectからPort、database、外部serviceを呼び出さないこと。
- Aggregate Rootは外部処理に必要な値を生成して返し、Portの呼び出しはusecaseまたはDomain Serviceが行うこと。
- Aggregate Rootのmethodには、そのAggregate Rootと所有するEntity、Value Objectだけに閉じた処理を書くこと。
- 別のAggregateを参照して生成や変更を行う処理は、Aggregate RootのmethodではなくDomain Serviceとして定義すること。

## Value Object

- EntityではないValue Objectはstructとして定義すること。
- Value Objectには`NewXxx` constructorを定義し、生成時はconstructorを使うこと。
- Value Object以外の型には、constructorを機械的に追加しないこと。
- Value Objectのconstructorは、同じファイルで新しいstruct値を生成して返すこと。
- named primitive typeは、許可する値をtyped constantで定義すること。
- named primitive typeのconstructorは入力値を検証し、外部入力を直接castしないこと。
- defaultを持つValue Objectは、不正値を明文化されたdefaultへ変換すること。
- 不正値を拒否するValue Objectは、constructorからerrorを返すこと。
- 検証済みの値を各layerで重複検証しないこと。
- Value Objectの値を変更する操作は、そのValue Object自身のmethodとして定義すること。
- Value Objectの変更methodは値レシーバにし、元のinstanceを変更せず、変更後の新しいinstanceをconstructor経由で返すこと。ポインタレシーバにしないこと。
- 値をそのまま返すだけのgetterや`String` methodを追加しないこと。

## Domain ServiceとPort

- Domain Serviceは、複数のAggregateにまたがるdomain logicであり、Value Object、Entity、Aggregate Rootでは表現できない場合に限って定義すること。
- Domain Serviceを定義する前に、その処理をValue Object、Entity、Aggregate Rootのmethodとして表現できないか必ず検討すること。
- 単一のAggregate Rootまたはその所有Entityだけに属する処理を、Domain Serviceとして定義しないこと。
- Domain Serviceでは、必要なdatabase処理や外部service処理を実行してよい。
- Domain Serviceには、その処理に必要な最小限の値を渡すこと。少数の値を得るだけのためにAggregate Root全体を渡さないこと。
- Domain ServiceはPortを呼び出してもよいが、Portを呼び出せることだけを理由に定義してはならない。Domain ServiceはValue Object、Entity、Aggregate Rootでは表現できないdomain logicを表現する場合に限って定義すること。Port呼び出しを伴わないdomain logicにも同じ規則を適用すること。
- Domain Serviceは1ファイルに1関数だけ定義し、ファイル名をDomain Service名に一致させること。
- Domain Service名には`Service`接尾辞を付け、ファイル名にも`service`を含めること。例: `CreateContainersService` は `create_containers_service.go` に1関数だけ定義すること。
- Domain ServiceをAggregate Rootのforwarding methodとして追加しないこと。
- Application Serviceが必要なPortをFactoryから生成すること。Domain Serviceが外部処理を必要とする場合は、当該Serviceと同じファイルに必要最小限のPortを宣言し、Application Serviceから渡すこと。
- interfaceは実装側ではなく利用側に定義し、利用側が必要とするmethodだけを含めること。
- UseCaseが使うPortはusecase packageへ定義すること。Domain Serviceが使うPortは、そのServiceと同じファイルのdomain packageへ定義すること。

## テスト

- test coverageは100%を目標とすること。
- 現在要求されている振る舞いと、明文化された不変条件だけをtestすること。
- 同じ振る舞いを複数layerで重複してtestしないこと。
- 実装詳細、仕様で順序保証されていない内部呼出順、単純な代入、言語機能そのものをtestしないこと。
- 削除した機能が利用できなくなったことだけを確認するtestを追加しないこと。
- 使用禁止、未対応、削除済みであることは、不要なsymbolの削除、compile、静的解析、dead code解析で確認すること。
- coverageを上げる目的だけのtestや、現在の要件にない入力を網羅する過剰なtable testを追加しないこと。
- 正常系、境界値、error系は、現在の仕様または不変条件に必要なcaseだけを選ぶこと。
- production codeから削除した処理だけを対象とするtestは、一緒に削除すること。

## 変更後の確認

- 変更したコードについて、requestとrepositoryの事実に反する仮定がないか確認すること。
- 存在しないAPI、method、command、設定を追加していないか確認すること。
- scope creep、不要なfallback、互換処理、重複validation、未使用helper、callerのないsymbolを削除すること。
- 近接コードの命名と構造を確認し、repositoryに存在しない一般論のarchitectureを持ち込まないこと。
- formatter、test、race test、static analysis、diff checkを実行すること。
