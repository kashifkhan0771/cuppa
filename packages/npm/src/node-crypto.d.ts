// Just the one function we use from Node, so we need no @types/node.
declare module "node:crypto" {
  export function createHash(algorithm: "sha256"): {
    update(data: string): { digest(): Uint8Array };
  };
}
