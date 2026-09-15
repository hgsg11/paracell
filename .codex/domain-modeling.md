# Domain Modeling Rules

- Treat the aggregate root as the only boundary for accessing or changing its owned entities. Code outside the aggregate root must not directly read or mutate owned entity fields; expose an operation-specific aggregate method instead.
- Define behavior that belongs to an entity or aggregate as a method on that type. If the behavior changes its receiver, use a pointer receiver and mutate the existing instance. Do not model mutation as a free function that accepts a value and returns a changed copy.
- Keep domain services as independent functions only when the operation does not belong to a single entity or aggregate. A domain service must update aggregate-owned state through pointer-receiver methods on the aggregate root.
- Put every domain object in its own responsibility-specific file: one aggregate, entity, value object, or Port DTO per file. Keep that object's constructor and object-specific methods in the same file. Do not group multiple objects into one file.
- For a value object that is not an entity, define its `NewXxx` constructor in the same file as the value-object struct. The constructor must build and return a new struct value. External input must pass through the constructor, including validation required by the value object's invariants.
- Define behavior that changes a value object as a value-receiver method in that value object's file. The method must leave its receiver unchanged and return a newly constructed value. Do not turn a responsibility explicitly assigned to a domain service into a value-object method.
