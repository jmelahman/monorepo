import React from 'react';
import { inject, observer } from "mobx-react";

import { Resource } from "./Store";
import barracks from './barracks.jpg';
import blacksmith from './blacksmith.jpg';
import church from './church.jpg';
import marketplace from './marketplace.jpg';
import tavern from './tavern.jpg';
import tower from './tower.jpg';
import wood from './wood.png';
import sulfur from './sulfur.png';
import crystal from './crystal.png';
import mercury from './mercury.png';
import ore from './ore.png';
import gems from './gems.png';
import gold from './gold.png';

function CastlePage() {
  return (
    <div className="castle">
      <div className="castle__layout">
        <div className="row">
          <Building title="Mage Guild" description="Arcane" owned={true}/>
          <Building title="Marketplace" image={marketplace} description="Purchase wares" owned={false}/>
          <Building title="Blacksmith" image={blacksmith} description="Upgrade equipment" owned={true}/>
        </div>
        <div className="row">
          <Building title="Scout Post" image={tower} description="Improve scouting" owned={false}/>
          <Building title="Inn" description="Recover" owned={true}/>
          <Building title="Church" image={church} description="Heal" owned={true}/>
        </div>
        <div className="row">
          <Building title="Training Grounds" description="Train heroes" owned={true}/>
          <Building title="Barracks" image={barracks} description="Update party" owned={true}/>
          <Building title="Tavern" image={tavern} description="Recruit Heroes" owned={true}/>
        </div>
      </div>
      <div className="dashboard">
        <div className="dashboard__left">
          <div className="party">
            <Hero name="Foo"/>
            <Hero name="Bar"/>
            <Hero name="Simone"/>
          </div>
        </div>
        <div className="dashboard__right">
          <div className="resources">
            <div className="row">
              <ResourceDisplay image={wood} resource={Resource.Wood} />
              <ResourceDisplay image={sulfur} resource={Resource.Sulfur}/>
            </div>
            <div className="row">
              <ResourceDisplay image={crystal} resource={Resource.Crystal}/>
              <ResourceDisplay image={mercury} resource={Resource.Mercury}/>
            </div>
            <div className="row">
              <ResourceDisplay image={ore} resource={Resource.Ore}/>
              <ResourceDisplay image={gems} resource={Resource.Gems}/>
            </div>
            <div className="row">
              <ResourceDisplay image={gold} resource={Resource.Gold}/>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

interface ResourceDisplayProps {
  resource: Resource;
  image: any;
  resources?: any;
}

@inject('resources')
@observer
class ResourceDisplay extends React.PureComponent<ResourceDisplayProps> {
  render() {
    const { image, resource, resources } = this.props;
    return (
      <div className={`resource ${resource}`}>
        <img src={image} title={resource.charAt(0).toUpperCase() + resource.slice(1)} className="resource__icon"/>
        <span className="resource__count">{resources[resource]}</span>
      </div>
    )
  }
}

interface BuildingProps {
  title: string;
  description: string;
  owned: boolean;
  image?: any;
  resources?: any;
}

@inject('resources')
class Building extends React.PureComponent<BuildingProps> {
  handleClick = () => {
    const { resources } = this.props;
    resources[Resource.Gold] -= 100;
  };

  render() {
    const { description, image, title, owned  } = this.props;
    return (
      <div className={`building ${owned ? 'owned' : 'unowned'}`}>
        <button onClick={this.handleClick} className="placeholder" title={description}>
          <img alt="tavern" src={image || tavern} className="building__image"/>
        </button>
        <div className="title">
          <span>{title}</span>
        </div>
      </div>
    )
  }
}

interface HeroProps {
  name: string;
}

function Hero(props: HeroProps) {
  return <div className="hero">{props.name}</div>
}

export default CastlePage;