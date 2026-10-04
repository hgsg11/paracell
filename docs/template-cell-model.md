# Template and runtime cell model

The template describes runtime cells; only the CommanderCell is selected and
listed as a work item. TargetCell and DependencyCell are independent runtime
cells; the CommanderCell references them by ID and does not own or contain them.
This is a domain relationship diagram, not a statement about the SQLite storage
layout.

```mermaid
erDiagram
    TEMPLATE ||--o| COMMANDER_CELL_SPEC : defines
    COMMANDER_CELL_SPEC ||--|| WORKSPACE_TEMPLATE : configures
    COMMANDER_CELL_SPEC ||--o{ TARGET_CELL_SPEC : defines
    COMMANDER_CELL_SPEC ||--o{ DEPENDENCY_CELL_SPEC : defines
    TARGET_CELL_SPEC }o--o{ DEPENDENCY_CELL_SPEC : depends_on

    COMMANDER_CELL ||--|| WORKSPACE : owns
    COMMANDER_CELL ||--o{ TARGET_CELL : references
    COMMANDER_CELL ||--o{ DEPENDENCY_CELL : references
    TARGET_CELL }o--o{ DEPENDENCY_CELL : depends_on
    TARGET_CELL o|--o| SOURCE : uses
    TARGET_CELL o|--o| CONTAINER : runs
    DEPENDENCY_CELL ||--|| CONTAINER : runs
```

Every TargetCell has at least one of Source or Container, while either may be
present alone or both may be present. A DependencyCell has exactly one
dependency-mode Container. CommanderCell owns the shared Workspace and
workflow state; it is not the aggregate root of the target/dependency cells.
