const shellUnsafePattern = /[\s"'\\$`!#&*();<>?[\]^{|}~]/;

export function quoteShellArgs(args: string[]) {
  return args.map(quoteShellArg).join(" ");
}
