const shellUnsafePattern = /[\s"'\\$`!#&*();<>?[\]^{|}~]/;

function quoteShellArg(value: string) {
  if (value === "") return "''";
  if (shellUnsafePattern.test(value)) {
    return `'${value.replace(/'/g, `'\\''`)}'`;
  }
  return value;
}

export function quoteShellArgs(args: string[]) {
  return args.map(quoteShellArg).join(" ");
}
