
export type ConfigValue = {
	value: string;
	secret: boolean;
}

export type ConfigMap = Map<string, ConfigValue>