import * as React from 'react';
import { Redirect, Route, Switch } from 'react-router-dom';

import BuildListPage from './BuildListPage';
import BuildDetailsPage from './BuildDetailsPage';
import BuildCreatePage from './BuildCreatePage';
import DinosaurListPage from './DinosaurListPage';
import DinosaurDetailsPage from './DinosaurDetailsPage';
import DinosaurCreatePage from './DinosaurCreatePage';
import EntityDefinitionListPage from './EntityDefinitionListPage';
import EntityDefinitionDetailsPage from './EntityDefinitionDetailsPage';
import EntityDefinitionCreatePage from './EntityDefinitionCreatePage';
import FieldDefinitionListPage from './FieldDefinitionListPage';
import FieldDefinitionDetailsPage from './FieldDefinitionDetailsPage';
import FieldDefinitionCreatePage from './FieldDefinitionCreatePage';
import FossilListPage from './FossilListPage';
import FossilDetailsPage from './FossilDetailsPage';
import FossilCreatePage from './FossilCreatePage';
import ProjectListPage from './ProjectListPage';
import ProjectDetailsPage from './ProjectDetailsPage';
import ProjectCreatePage from './ProjectCreatePage';
import ProjectDashboard from './ProjectDashboard';
import RelationshipListPage from './RelationshipListPage';
import RelationshipDetailsPage from './RelationshipDetailsPage';
import RelationshipCreatePage from './RelationshipCreatePage';
import ScientistListPage from './ScientistListPage';
import ScientistDetailsPage from './ScientistDetailsPage';
import ScientistCreatePage from './ScientistCreatePage';

const App: React.FC = () => (
  <Switch>
    <Route exact path="/trex-console/projects" component={ProjectListPage} />
    <Route exact path="/trex-console/projects/create" component={ProjectCreatePage} />
    <Route exact path="/trex-console/projects/:id/dashboard" component={ProjectDashboard} />
    <Route exact path="/trex-console/projects/:id" component={ProjectDetailsPage} />
    <Route exact path="/trex-console/entity-definitions" component={EntityDefinitionListPage} />
    <Route exact path="/trex-console/entity-definitions/create" component={EntityDefinitionCreatePage} />
    <Route exact path="/trex-console/entity-definitions/:id" component={EntityDefinitionDetailsPage} />
    <Route exact path="/trex-console/field-definitions" component={FieldDefinitionListPage} />
    <Route exact path="/trex-console/field-definitions/create" component={FieldDefinitionCreatePage} />
    <Route exact path="/trex-console/field-definitions/:id" component={FieldDefinitionDetailsPage} />
    <Route exact path="/trex-console/relationships" component={RelationshipListPage} />
    <Route exact path="/trex-console/relationships/create" component={RelationshipCreatePage} />
    <Route exact path="/trex-console/relationships/:id" component={RelationshipDetailsPage} />
    <Route exact path="/trex-console/builds" component={BuildListPage} />
    <Route exact path="/trex-console/builds/create" component={BuildCreatePage} />
    <Route exact path="/trex-console/builds/:id" component={BuildDetailsPage} />
    <Route exact path="/trex-console/dinosaurs" component={DinosaurListPage} />
    <Route exact path="/trex-console/dinosaurs/create" component={DinosaurCreatePage} />
    <Route exact path="/trex-console/dinosaurs/:id" component={DinosaurDetailsPage} />
    <Route exact path="/trex-console/fossils" component={FossilListPage} />
    <Route exact path="/trex-console/fossils/create" component={FossilCreatePage} />
    <Route exact path="/trex-console/fossils/:id" component={FossilDetailsPage} />
    <Route exact path="/trex-console/scientists" component={ScientistListPage} />
    <Route exact path="/trex-console/scientists/create" component={ScientistCreatePage} />
    <Route exact path="/trex-console/scientists/:id" component={ScientistDetailsPage} />
    <Redirect to="/trex-console/projects" />
  </Switch>
);

export default App;
