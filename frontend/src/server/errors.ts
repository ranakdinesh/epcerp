// Shared application failure type; each module supplies its own messages.
export class AppError extends Error {
  constructor(public status: number, message: string) { super(message); }
}
