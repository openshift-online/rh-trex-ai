import * as React from 'react';
import { useHistory } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Form,
  FormGroup,
  PageSection,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';

const DinosaurCreatePage: React.FC = () => {
  const history = useHistory();
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const [species, setSpecies] = React.useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    const body: Record<string, unknown> = {};
    if (species) body['species'] = species;

    try {
      const api = createAPIClient();
      const created = await api.dinosaurs.create(body);
      history.push(`/trex-console/dinosaurs/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/dinosaurs')}
            component="button"
          >
            Dinosaurs
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Create Dinosaur</Title>
      </PageSection>
      <PageSection>
        {error && <Alert variant="danger" title="Error" isInline>{error}</Alert>}
        <Form onSubmit={handleSubmit} style={{ maxWidth: 600 }}>
          <FormGroup
            label="Species"
            fieldId="field-species"
            isRequired={ true}
          >
            <TextInput
              id="field-species"
              type="text"
              value={ species}
              onChange={(_e, val) => setSpecies(val)}
              placeholder="Enter Species"
              isRequired={ true}
            />
          </FormGroup>
          <ActionGroup>
            <Button type="submit" variant="primary" isLoading={submitting} isDisabled={submitting}>
              Create
            </Button>
            <Button
              variant="link"
              onClick={() => history.push('/trex-console/dinosaurs')}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
};

export default DinosaurCreatePage;
