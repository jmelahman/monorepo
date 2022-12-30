import { observable } from "mobx";

export enum Resource {
    Wood = 'Wood',
    Sulfar = 'Sulfar',
    Crystal = 'Crystal',
    Mercury = 'Mercury',
    Ore = 'Ore',
    Gems = 'Gems',
    Gold = 'Gold',
}

export const resources = observable({
    [Resource.Wood]: 20,
    [Resource.Sulfar]: 10,
    [Resource.Crystal]: 10,
    [Resource.Mercury]: 10,
    [Resource.Ore]: 20,
    [Resource.Gems]: 10,
    [Resource.Gold]: 2000,
});
