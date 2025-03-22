// Import the component through a long chain
import { DeepComponent as DeepComponent1 } from "./path1/intermediate";

// Also import the same component through a shorter path
import { DeepComponent as DeepComponent2 } from "./path2/shortcut";

// Re-export both (should be the same component)
export { DeepComponent1, DeepComponent2 };
